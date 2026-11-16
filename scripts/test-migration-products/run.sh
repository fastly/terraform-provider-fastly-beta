#!/usr/bin/env bash

# End-to-end test for fastly-tf-migrate's configurable product migration
# (Bot Management, DDoS Protection, NGWAF) with non-default settings.
#
# Usage: run.sh [enabled|disabled|all]   (default: all)
#
# Each scenario:
#   1. Create a legacy fastly_service_vcl with product_enablement (enabled = true
#      or false on every configurable product) using the legacy fastly/fastly
#      provider, then refresh state
#   2. Snapshot the live product state and configuration through the API
#   3. fastly-tf-migrate analyze -> migrate --dry-run -> migrate --write
#   4. Switch to the locally built dual-model provider
#   5. terraform plan: every action must be no-op/read (imports only), so
#      creates, deletes, replacements and in-place updates all fail the run
#   6. terraform apply, then a final plan that must be a no-op (exit code 0)
#   7. Re-snapshot the live products; the snapshot must be identical, and in the
#      enabled scenario contentguard, mode, workspace_id and traffic_ramp must
#      still hold their non-default values
#   8. Inspect the provider's HTTP calls during steps 5-6: no mutating request
#      (anything but GET/HEAD) may reach a product or NGWAF API path
#   9. terraform destroy
#
# The disabled scenario instead expects the migrator to refuse: importing a
# disabled configurable product plans an in-place update that calls the disable
# API, so `migrate --dry-run` and `--write` must fail without touching any file,
# and the legacy service is then destroyed with the legacy provider.
#
# Requires FASTLY_API_TOKEN. A throwaway NGWAF workspace (mode "off") is created
# per scenario and deleted afterwards, unless FASTLY_TEST_NGWAF_WORKSPACE_ID is
# set, in which case that workspace is used for every scenario and left alone.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-products.XXXXXX")"
PROVIDER_DIR="$WORK_DIR/provider"
BACKUP_DIR="$WORK_DIR/backups"
MIGRATOR="$WORK_DIR/fastly-tf-migrate"

NAME_PREFIX="tf-test-migprod-$$"
LEGACY_PROVIDER_VERSION="${LEGACY_PROVIDER_VERSION:-9.5.0}"
API_BASE="https://api.fastly.com"
PRODUCTS=(bot_management ddos_protection ngwaf)

CONTENTGUARD="on"
DDOS_MODE="block"
NGWAF_TRAFFIC_RAMP=25

PHASE="setup"
KEEP_WORK_DIR="${KEEP_WORK_DIR:-}"
PRESET_WORKSPACE_ID="${FASTLY_TEST_NGWAF_WORKSPACE_ID:-}"

# Registered as resources are created so the cleanup trap can undo them.
PROJECT_DIRS=()
CREATED_WORKSPACES=()

# Per-scenario state.
SCENARIO=""
PRODUCT_ENABLED=""
SERVICE_NAME=""
SCENARIO_DIR=""
PROJECT_DIR=""
WORKSPACE_ID=""
SERVICE_ID=""

log_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $*"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()    { echo -e "\n${BLUE}===${NC} $* ${BLUE}===${NC}\n"; }

use_legacy_provider() {
    unset TF_CLI_CONFIG_FILE
    # The legacy provider reads FASTLY_API_KEY; the dual-model one reads FASTLY_API_TOKEN.
    export FASTLY_API_KEY="$FASTLY_API_TOKEN"
}

use_ga_provider() {
    unset FASTLY_API_KEY
    export TF_CLI_CONFIG_FILE="$WORK_DIR/.terraformrc"
}

api_get() {
    # Prints "<body>\n<status>". The key is only ever passed as a header.
    curl -s -w '\n%{http_code}' -H "Fastly-Key: $FASTLY_API_TOKEN" "$API_BASE$1"
}

delete_workspace() {
    local status
    status="$(curl -s -o /dev/null -w '%{http_code}' -X DELETE \
        -H "Fastly-Key: $FASTLY_API_TOKEN" \
        "$API_BASE/ngwaf/v1/workspaces/$1")"
    case "$status" in
        200|202|204) log_info "Deleted workspace $1" ;;
        *) return 1 ;;
    esac
}

cleanup() {
    local exit_code=$?
    trap - EXIT INT TERM

    if [ $exit_code -ne 0 ]; then
        log_error "Script failed in scenario '${SCENARIO:-none}', phase '$PHASE' with exit code $exit_code"
        log_step "Emergency cleanup"
        local dir
        for dir in ${PROJECT_DIRS[@]+"${PROJECT_DIRS[@]}"}; do
            if [ -f "$dir/terraform.tfstate" ]; then
                (cd "$dir" && terraform destroy -auto-approve -input=false) \
                    || log_warning "terraform destroy failed in $dir; falling back to API sweep"
            fi
        done
        (cd "$REPO_ROOT" && go run ./scripts/cleanup-test-services -prefix "$NAME_PREFIX") \
            || log_warning "API sweep failed; check for leftover services named $NAME_PREFIX*"
    fi

    # Runs on success too: the services are gone by now, so the workspaces are unlinked.
    local workspace
    for workspace in ${CREATED_WORKSPACES[@]+"${CREATED_WORKSPACES[@]}"}; do
        delete_workspace "$workspace" || log_warning "Could not delete workspace $workspace; remove it manually"
    done

    if [ -n "$KEEP_WORK_DIR" ] || [ $exit_code -ne 0 ]; then
        log_info "Work directory kept at: $WORK_DIR"
    else
        rm -rf "$WORK_DIR"
    fi
    exit $exit_code
}
trap cleanup EXIT INT TERM

check_prerequisites() {
    PHASE="prerequisites"
    log_step "Checking prerequisites"

    if [ -z "${FASTLY_API_TOKEN:-}" ]; then
        log_error "FASTLY_API_TOKEN is not set"
        exit 1
    fi
    for cmd in terraform go jq curl; do
        if ! command -v "$cmd" > /dev/null 2>&1; then
            log_error "Required command '$cmd' not found"
            exit 1
        fi
    done
    log_success "Prerequisites satisfied"
}

build_binaries() {
    PHASE="build"
    log_step "Building provider and migrator"

    mkdir -p "$PROVIDER_DIR"
    cd "$REPO_ROOT"
    # The migrator keeps source = "fastly/fastly", so the local build must be
    # served under that address; the binary name is derived from the type.
    go build -o "$PROVIDER_DIR/terraform-provider-fastly"
    go build -o "$MIGRATOR" ./cmd/fastly-tf-migrate

    cat > "$WORK_DIR/.terraformrc" << EOF
provider_installation {
  dev_overrides {
    "fastly/fastly" = "$PROVIDER_DIR"
  }
  direct {}
}
EOF
    log_success "Built provider and migrator"
}

create_workspace() {
    PHASE="workspace"
    if [ -n "$PRESET_WORKSPACE_ID" ]; then
        WORKSPACE_ID="$PRESET_WORKSPACE_ID"
        log_info "Using existing NGWAF workspace from FASTLY_TEST_NGWAF_WORKSPACE_ID"
        return
    fi
    log_step "Creating throwaway NGWAF workspace"

    local response status body
    response="$(curl -s -w '\n%{http_code}' -X POST \
        -H "Fastly-Key: $FASTLY_API_TOKEN" -H "Content-Type: application/json" \
        -d "{\"name\":\"$SERVICE_NAME\",\"mode\":\"off\"}" \
        "$API_BASE/ngwaf/v1/workspaces")"
    status="${response##*$'\n'}"
    body="${response%$'\n'*}"
    if [ "$status" != "200" ] && [ "$status" != "201" ]; then
        log_error "Workspace creation failed (HTTP $status): $body"
        exit 1
    fi
    WORKSPACE_ID="$(echo "$body" | jq -r '.id')"
    if [ -z "$WORKSPACE_ID" ] || [ "$WORKSPACE_ID" = "null" ]; then
        log_error "Workspace response had no id: $body"
        exit 1
    fi
    CREATED_WORKSPACES+=("$WORKSPACE_ID")
    log_success "Created workspace $WORKSPACE_ID"
}

write_legacy_config() {
    PHASE="legacy-config"
    log_step "Writing legacy configuration"

    cat > "$PROJECT_DIR/main.tf" << EOF
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = ">= $LEGACY_PROVIDER_VERSION, < 20.0.0"
    }
  }
}

provider "fastly" {}

resource "fastly_service_vcl" "demo" {
  name          = "$SERVICE_NAME"
  force_destroy = true

  domain {
    name = "$SERVICE_NAME.example.com"
  }

  backend {
    address = "example.com"
    name    = "origin"
    port    = 443
  }

  product_enablement {
    websockets = true

    bot_management {
      enabled      = $PRODUCT_ENABLED
      contentguard = "$CONTENTGUARD"
    }

    ddos_protection {
      enabled = $PRODUCT_ENABLED
      mode    = "$DDOS_MODE"
    }

    ngwaf {
      enabled      = $PRODUCT_ENABLED
      workspace_id = "$WORKSPACE_ID"
      traffic_ramp = $NGWAF_TRAFFIC_RAMP
    }
  }
}
EOF
}

apply_legacy() {
    PHASE="legacy-apply"
    log_step "Applying legacy configuration"

    cd "$PROJECT_DIR"
    use_legacy_provider
    terraform init -input=false
    terraform apply -auto-approve -input=false
    # Persists state at schema 1, a migrator prerequisite.
    terraform apply -refresh-only -auto-approve -input=false

    SERVICE_ID="$(terraform show -json | jq -r '.values.root_module.resources[] | select(.address=="fastly_service_vcl.demo") | .values.id')"
    if [ -z "$SERVICE_ID" ] || [ "$SERVICE_ID" = "null" ]; then
        log_error "Could not read the legacy service ID from state"
        exit 1
    fi
    log_success "Legacy service created: $SERVICE_ID"
}

# snapshot_products FILE records status and body of every product status and
# configuration endpoint as one JSON object.
snapshot_products() {
    local out="$1" product suffix response status body body_json state='{}'
    for product in "${PRODUCTS[@]}"; do
        for suffix in "" "/configuration"; do
            response="$(api_get "/enabled-products/v1/$product/services/$SERVICE_ID$suffix")"
            status="${response##*$'\n'}"
            body="${response%$'\n'*}"
            body_json="$(echo "$body" | jq -c . 2>/dev/null || printf '%s' "$body" | jq -Rs .)"
            state="$(jq -c --arg k "$product$suffix" --argjson s "$status" --argjson b "$body_json" \
                '. + {($k): {status: $s, body: $b}}' <<< "$state")"
        done
    done
    jq -S . <<< "$state" > "$out"
}

# assert_snapshot FILE KEY JQ_PATH EXPECTED compares a value in a snapshot.
assert_snapshot() {
    local file="$1" key="$2" path="$3" expected="$4" actual
    actual="$(jq -r --arg k "$key" ".[\$k].$path" "$file")"
    if [ "$actual" != "$expected" ]; then
        log_error "$key $path = '$actual' in $(basename "$file"), expected '$expected'"
        exit 1
    fi
}

assert_snapshot_settings() {
    local file="$1"
    if [ "$PRODUCT_ENABLED" = "true" ]; then
        assert_snapshot "$file" "bot_management" "status" "200"
        assert_snapshot "$file" "bot_management/configuration" "body.configuration.contentguard" "$CONTENTGUARD"
        assert_snapshot "$file" "ddos_protection/configuration" "body.configuration.mode" "$DDOS_MODE"
        assert_snapshot "$file" "ngwaf/configuration" "body.configuration.workspace_id" "$WORKSPACE_ID"
        assert_snapshot "$file" "ngwaf/configuration" "body.configuration.traffic_ramp" "$NGWAF_TRAFFIC_RAMP"
    else
        local product
        for product in "${PRODUCTS[@]}"; do
            if [ "$(jq -r --arg k "$product" '.[$k].status' "$file")" = "200" ]; then
                log_error "$product is unexpectedly enabled in $(basename "$file")"
                exit 1
            fi
        done
    fi
}

run_migrator() {
    PHASE="migrate"
    log_step "Running fastly-tf-migrate"

    cd "$PROJECT_DIR"
    "$MIGRATOR" analyze .
    "$MIGRATOR" migrate --dry-run . | tee "$SCENARIO_DIR/dry-run.diff"

    local product
    for product in "${PRODUCTS[@]}"; do
        if ! grep -q "fastly_service_product_$product" "$SCENARIO_DIR/dry-run.diff"; then
            log_error "dry-run diff does not mention fastly_service_product_$product"
            exit 1
        fi
    done

    "$MIGRATOR" migrate --write --backup-dir "$BACKUP_DIR/$SCENARIO" .

    if grep -q "product_enablement" main.tf; then
        log_error "legacy product_enablement block still present after migration"
        exit 1
    fi
    local expected
    for expected in \
        "enabled      = $PRODUCT_ENABLED" \
        "contentguard = \"$CONTENTGUARD\"" \
        "\"$DDOS_MODE\"" \
        "workspace_id = \"$WORKSPACE_ID\"" \
        "traffic_ramp = $NGWAF_TRAFFIC_RAMP"; do
        if ! grep -q -- "$expected" migration.tf; then
            log_error "migration.tf is missing: $expected"
            exit 1
        fi
    done
    log_success "Migration written; settings preserved in migration.tf"
}

# provider_calls LOG prints "METHOD PATH" for each HTTP request the provider
# made, from the go-fastly debug dump (the API key header is removed from it).
provider_calls() {
    # The dump reaches the trace as a quoted, escaped string (\"GET /path HTTP/1.1...).
    grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$1" | sed -E 's/^.*"([A-Z]+ )/\1/' | sort | uniq
}

assert_no_mutating_product_calls() {
    local log="$1" calls mutating
    calls="$(provider_calls "$log")"
    if ! echo "$calls" | grep -q '^GET /enabled-products/'; then
        log_error "No product GET requests captured in $log; HTTP capture is not working"
        exit 1
    fi
    mutating="$(echo "$calls" | grep -v -E '^(GET|HEAD) ' || true)"
    if [ -n "$mutating" ]; then
        log_info "Mutating provider requests during migration:"
        echo "$mutating"
    fi
    if echo "$mutating" | grep -q -E ' /(enabled-products|ngwaf)/'; then
        log_error "Migration invoked mutating product API calls:"
        echo "$mutating" | grep -E ' /(enabled-products|ngwaf)/'
        exit 1
    fi
    log_success "No mutating product API calls during migration ($(echo "$calls" | wc -l | tr -d ' ') distinct requests observed)"
}

plan_and_apply_ga() {
    PHASE="ga-plan"
    log_step "Switching to the dual-model provider"

    cd "$PROJECT_DIR"
    use_ga_provider
    # The local build is served via dev_overrides and no 20.x release exists on the
    # registry yet, so `terraform init` cannot resolve the >= 20.0.0 constraint.
    # Overridden providers need no installation, so skip init and drop the stale
    # legacy lock entry.
    rm -f .terraform.lock.hcl
    terraform validate

    local trace="$SCENARIO_DIR/ga-trace.log"
    : > "$trace"
    # Keeps the debug dump of every provider HTTP request in the trace.
    export FASTLY_DEBUG_MODE=true TF_LOG=TRACE TF_LOG_PATH="$trace"

    terraform plan -out="$SCENARIO_DIR/migrate.plan" -input=false
    terraform show -json "$SCENARIO_DIR/migrate.plan" > "$SCENARIO_DIR/migrate.plan.json"

    # Adoption must be imports only: an update on a product can enable, disable,
    # reconfigure or relink it, and a changed service would be mutated too.
    local unsafe
    unsafe="$(jq -r '[.resource_changes[]?
        | select(.change.actions | all(. == "no-op" or . == "read") | not)
        | "\(.address): \(.change.actions | join(","))"] | .[]' "$SCENARIO_DIR/migrate.plan.json")"
    if [ -n "$unsafe" ]; then
        log_error "Plan proposes more than imports (expected only no-op/read actions):"
        echo "$unsafe"
        exit 1
    fi

    local importing
    importing="$(jq '[.resource_changes[]? | select(.change.importing != null)] | length' "$SCENARIO_DIR/migrate.plan.json")"
    if [ "$importing" -ne 4 ]; then
        log_error "Expected 4 product imports in the plan, found $importing"
        exit 1
    fi
    log_success "Plan is imports only: 4 products, no creates, updates, deletes or replacements"

    PHASE="ga-apply"
    terraform apply -input=false "$SCENARIO_DIR/migrate.plan"

    PHASE="ga-noop-plan"
    log_step "Final plan must be a no-op"
    local rc=0
    terraform plan -detailed-exitcode -input=false || rc=$?
    if [ $rc -ne 0 ]; then
        log_error "Final plan exit code $rc (0 = no changes, 2 = changes)"
        exit 1
    fi
    log_success "Final plan is a no-op"

    unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH
    assert_no_mutating_product_calls "$trace"
}

# expect_migration_blocked verifies that the migrator refuses a configuration
# with disabled configurable products, writes nothing, and names the reason.
expect_migration_blocked() {
    PHASE="migrate-blocked"
    log_step "Migrator must refuse disabled configurable products"

    cd "$PROJECT_DIR"
    local before after product output
    before="$(find . -path ./.terraform -prune -o -type f -name '*.tf' -print | sort | xargs shasum)"

    for mode in --dry-run --write; do
        if output="$("$MIGRATOR" migrate "$mode" . 2>&1)"; then
            log_error "migrate $mode succeeded for disabled configurable products"
            echo "$output"
            exit 1
        fi
        for product in "${PRODUCTS[@]}"; do
            if ! grep -q "product_enablement.$product.enabled = false cannot be migrated safely" <<< "$output"; then
                log_error "migrate $mode did not report the disabled $product block:"
                echo "$output"
                exit 1
            fi
        done
    done

    after="$(find . -path ./.terraform -prune -o -type f -name '*.tf' -print | sort | xargs shasum)"
    if [ "$before" != "$after" ] || [ -e migration.tf ]; then
        log_error "A blocked migration changed the configuration"
        exit 1
    fi
    log_success "Migration blocked for all three disabled products; no files changed"
}

run_scenario() {
    SCENARIO="$1"
    case "$SCENARIO" in
        enabled)  PRODUCT_ENABLED=true ;;
        disabled) PRODUCT_ENABLED=false ;;
        *) log_error "Unknown scenario '$SCENARIO' (expected enabled, disabled or all)"; exit 1 ;;
    esac
    SERVICE_NAME="$NAME_PREFIX-$SCENARIO"
    SCENARIO_DIR="$WORK_DIR/$SCENARIO"
    PROJECT_DIR="$SCENARIO_DIR/project"
    mkdir -p "$PROJECT_DIR"
    PROJECT_DIRS+=("$PROJECT_DIR")

    log_step "Scenario: configurable products enabled = $PRODUCT_ENABLED"

    create_workspace
    write_legacy_config
    apply_legacy

    PHASE="snapshot-before"
    snapshot_products "$SCENARIO_DIR/products-before.json"
    assert_snapshot_settings "$SCENARIO_DIR/products-before.json"
    log_success "Captured live product configuration before migration"

    if [ "$PRODUCT_ENABLED" = "false" ]; then
        expect_migration_blocked
        PHASE="destroy"
        log_step "Destroying resources"
        cd "$PROJECT_DIR"
        use_legacy_provider
        terraform destroy -auto-approve -input=false
        log_success "Scenario '$SCENARIO' passed"
        return
    fi

    run_migrator
    plan_and_apply_ga

    PHASE="snapshot-after"
    snapshot_products "$SCENARIO_DIR/products-after.json"
    if ! diff -u "$SCENARIO_DIR/products-before.json" "$SCENARIO_DIR/products-after.json"; then
        log_error "Live product configuration changed during migration"
        exit 1
    fi
    assert_snapshot_settings "$SCENARIO_DIR/products-after.json"
    log_success "Live product configuration is identical before and after migration"

    PHASE="destroy"
    log_step "Destroying resources"
    cd "$PROJECT_DIR"
    terraform destroy -auto-approve -input=false
    log_success "Scenario '$SCENARIO' passed"
}

main() {
    local selection="${1:-all}"
    check_prerequisites
    build_binaries
    case "$selection" in
        all) run_scenario enabled; run_scenario disabled ;;
        *)   run_scenario "$selection" ;;
    esac
    log_success "Configurable product migration end-to-end test passed ($selection)"
}

main "$@"
