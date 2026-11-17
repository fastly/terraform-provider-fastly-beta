#!/usr/bin/env bash

# Live end-to-end tests for all automatically migrated CDN product enablements:
# seven simple boolean products and three configurable products.
#
# Usage: run.sh [all|simple|simple:PRODUCT|configurable|enabled|disabled|list]
#
# Each simple product runs on its own throwaway legacy VCL service (enabled=true)
# and must be adopted by one imported standalone resource with no API mutation.
# The existing configurable scenarios are retained: enabled adopts the three
# products plus WebSockets, disabled must fail before writing any Terraform HCL.
#
# Every successful migration runs analyze -> dry-run -> write -> Terraform
# plan/apply -> no-op plan, and checks live product state and provider API calls.
# Requires FASTLY_API_TOKEN and product entitlements on a non-production account.
# Compute product migration is deliberately excluded while its CLI rules remain
# blocked; that has separate prerequisites and acceptance coverage.

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
CONFIGURABLE_PRODUCTS=(bot_management ddos_protection ngwaf)
SIMPLE_PRODUCTS=(
    api_discovery brotli_compression domain_inspector image_optimizer
    log_explorer_insights origin_inspector websockets
)
PRODUCTS=()
EXPECTED_PRODUCTS=()
EXPECTED_IMPORTS=0

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
SCENARIO_KIND=""
SIMPLE_PRODUCT=""
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
        if [ -n "$SCENARIO" ]; then
            (cd "$REPO_ROOT" && go run ./scripts/cleanup-test-services -prefix "$NAME_PREFIX") \
                || log_warning "API sweep failed; check for leftover services named $NAME_PREFIX*"
        fi
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

    # Fastly requires every backend to have a shield before Image Optimizer
    # can be enabled. Use the POP from the repository's shielded-backend fixture.
    # Keep all other product scenarios on the original unshielded fixture.
    local backend_shield=""
    if [ "$SCENARIO_KIND" = "simple" ] && [ "$SIMPLE_PRODUCT" = "image_optimizer" ]; then
        backend_shield=$'\n    shield  = "amsterdam-nl"'
    fi

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
    port    = 443${backend_shield}
  }

  product_enablement {
EOF
    if [ "$SCENARIO_KIND" = "simple" ]; then
        printf '    %s = true\n' "$SIMPLE_PRODUCT" >> "$PROJECT_DIR/main.tf"
    else
        cat >> "$PROJECT_DIR/main.tf" << EOF
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
EOF
    fi
    printf '  }\n}\n' >> "$PROJECT_DIR/main.tf"
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

    # The CDN Auto state mover requires the persisted legacy VCL schema v1.
    local schema
    schema="$(terraform state pull | jq -r '[.resources[]? |
        select(.type == "fastly_service_vcl" and .name == "demo") |
        .instances[]?.schema_version] | unique |
        if length == 1 then .[0] else empty end')"
    if [ "$schema" != "1" ]; then
        log_error "Expected legacy VCL state schema 1, found '${schema:-missing}'"
        exit 1
    fi

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
        local suffixes=("")
        if [ "$SCENARIO_KIND" = "configurable" ]; then
            suffixes+=("/configuration")
        fi
        for suffix in "${suffixes[@]}"; do
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
    if [ "$SCENARIO_KIND" = "simple" ]; then
        assert_snapshot "$file" "$SIMPLE_PRODUCT" "status" "200"
    elif [ "$PRODUCT_ENABLED" = "true" ]; then
        local product
        for product in "${CONFIGURABLE_PRODUCTS[@]}"; do
            assert_snapshot "$file" "$product" "status" "200"
        done
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
    cp main.tf "$SCENARIO_DIR/main-before-dry-run.tf"
    "$MIGRATOR" migrate --dry-run . | tee "$SCENARIO_DIR/dry-run.diff"
    if ! cmp -s main.tf "$SCENARIO_DIR/main-before-dry-run.tf" || [ -e migration.tf ]; then
        log_error "migrate --dry-run modified the Terraform configuration"
        exit 1
    fi

    local product
    for product in "${EXPECTED_PRODUCTS[@]}"; do
        if ! grep -Fq "fastly_service_product_$product" "$SCENARIO_DIR/dry-run.diff"; then
            log_error "dry-run diff does not mention fastly_service_product_$product"
            exit 1
        fi
    done

    "$MIGRATOR" migrate --write --backup-dir "$BACKUP_DIR/$SCENARIO" .

    if grep -q "product_enablement" main.tf; then
        log_error "legacy product_enablement block still present after migration"
        exit 1
    fi
    if ! grep -Fq 'resource "fastly_service_cdn_auto" "demo"' main.tf; then
        log_error "main.tf does not contain the CDN Auto service"
        exit 1
    fi
    if ! grep -Fq 'from = fastly_service_vcl.demo' migration.tf; then
        log_error "migration.tf does not contain the VCL state move"
        exit 1
    fi
    for product in "${EXPECTED_PRODUCTS[@]}"; do
        if ! grep -Fq "resource \"fastly_service_product_$product\" \"demo\"" migration.tf; then
            log_error "migration.tf is missing the standalone resource for $product"
            exit 1
        fi
        if ! grep -Fq "to = fastly_service_product_$product.demo" migration.tf; then
            log_error "migration.tf is missing the import for $product"
            exit 1
        fi
    done

    local imports resources
    imports="$(grep -c -E '^[[:space:]]*import[[:space:]]*[{]' migration.tf || true)"
    resources="$(grep -c -E '^resource "fastly_service_product_' migration.tf || true)"
    if [ "$imports" -ne "$EXPECTED_IMPORTS" ] || [ "$resources" -ne "$EXPECTED_IMPORTS" ]; then
        log_error "Expected $EXPECTED_IMPORTS product resources/imports; found $resources resources and $imports imports"
        exit 1
    fi
    if ! grep -Eq '^[[:space:]]*enabled[[:space:]]*=[[:space:]]*true' migration.tf; then
        log_error "Expected enabled = true in the generated product configuration"
        exit 1
    fi

    if [ "$SCENARIO_KIND" = "configurable" ]; then
        local expected
        local checks=(
            "contentguard = \"$CONTENTGUARD\""
            "\"$DDOS_MODE\""
            "workspace_id = \"$WORKSPACE_ID\""
            "traffic_ramp = $NGWAF_TRAFFIC_RAMP"
        )
        for expected in "${checks[@]}"; do
            if ! grep -Fq -- "$expected" migration.tf; then
                log_error "migration.tf is missing: $expected"
                exit 1
            fi
        done
    fi
    log_success "Migration written with $EXPECTED_IMPORTS product resources/imports"
}

# provider_calls LOG prints "METHOD PATH" for each HTTP request the provider
# made, from the go-fastly debug dump (the API key header is removed from it).
provider_calls() {
    # The dump reaches the trace as a quoted, escaped string (\"GET /path HTTP/1.1...).
    grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$1" | sed -E 's/^.*"([A-Z]+ )/\1/' | sort | uniq
}

assert_no_mutating_product_calls() {
    local log="$1" calls mutating
    calls="$(provider_calls "$log")" || true
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
    if [ "$importing" -ne "$EXPECTED_IMPORTS" ]; then
        log_error "Expected $EXPECTED_IMPORTS product imports in the plan, found $importing"
        exit 1
    fi
    local previous
    previous="$(jq -r '.resource_changes[]? | select(.address == "fastly_service_cdn_auto.demo") | .previous_address // empty' "$SCENARIO_DIR/migrate.plan.json")"
    if [ "$previous" != "fastly_service_vcl.demo" ]; then
        log_error "Plan did not record the VCL state move (previous_address='$previous')"
        exit 1
    fi
    log_success "Plan moves VCL state and imports $EXPECTED_IMPORTS products with no infrastructure changes"

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

    local moved_id
    moved_id="$(terraform show -json | jq -r '.values.root_module.resources[] | select(.address == "fastly_service_cdn_auto.demo") | .values.id')"
    if [ "$moved_id" != "$SERVICE_ID" ]; then
        log_error "Service ID changed during migration: $SERVICE_ID -> $moved_id"
        exit 1
    fi

    local old_address product address
    old_address="$(terraform state list | grep -Fx 'fastly_service_vcl.demo' || true)"
    if [ -n "$old_address" ] || ! terraform state list | grep -Fxq 'fastly_service_cdn_auto.demo'; then
        log_error "Legacy VCL state address remains or CDN Auto address is missing"
        exit 1
    fi
    for product in "${EXPECTED_PRODUCTS[@]}"; do
        address="fastly_service_product_${product}.demo"
        if ! terraform state list | grep -Fxq "$address"; then
            log_error "Imported product state address missing: $address"
            exit 1
        fi
    done

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

is_simple_product() {
    local candidate="$1" product
    for product in "${SIMPLE_PRODUCTS[@]}"; do
        if [ "$candidate" = "$product" ]; then
            return 0
        fi
    done
    return 1
}

print_usage() {
    cat << EOF
Usage: $0 [all|simple|simple:PRODUCT|configurable|enabled|disabled|list]

  all                 All seven automatic simple CDN products, then both
                      configurable scenarios (requires all entitlements).
  simple              Every simple CDN product, each on its own service.
  simple:PRODUCT      One named simple CDN product; useful for individual
                      entitlements and debugging.
  configurable        Existing configured enabled/disabled scenarios.
  enabled             Configurable products enabled (4 imports including WebSockets).
  disabled            Configurable products disabled (migrator must block).
  list                Print covered product names without API credentials.

Simple CDN products: ${SIMPLE_PRODUCTS[*]}
Configurable CDN products: ${CONFIGURABLE_PRODUCTS[*]}
Compute product transforms are currently blocked in the CLI.
EOF
}

run_scenario() {
    SCENARIO="$1"
    SCENARIO_KIND="configurable"
    SIMPLE_PRODUCT=""
    PRODUCTS=("${CONFIGURABLE_PRODUCTS[@]}")
    EXPECTED_PRODUCTS=(websockets "${CONFIGURABLE_PRODUCTS[@]}")
    EXPECTED_IMPORTS=4
    case "$SCENARIO" in
        enabled)  PRODUCT_ENABLED=true ;;
        disabled) PRODUCT_ENABLED=false ;;
        simple:*)
            SCENARIO_KIND="simple"
            SIMPLE_PRODUCT="${SCENARIO#simple:}"
            if ! is_simple_product "$SIMPLE_PRODUCT"; then
                log_error "Unknown simple product '$SIMPLE_PRODUCT'"
                print_usage
                exit 2
            fi
            PRODUCTS=("$SIMPLE_PRODUCT")
            EXPECTED_PRODUCTS=("$SIMPLE_PRODUCT")
            EXPECTED_IMPORTS=1
            PRODUCT_ENABLED=true
            ;;
        *) log_error "Unknown scenario '$SCENARIO'"; print_usage; exit 2 ;;
    esac

    # Replace separators and underscores so test domains remain valid hostnames.
    SERVICE_NAME="$NAME_PREFIX-${SCENARIO//:/-}"
    SERVICE_NAME="${SERVICE_NAME//_/-}"
    SCENARIO_DIR="$WORK_DIR/$SCENARIO"
    PROJECT_DIR="$SCENARIO_DIR/project"
    mkdir -p "$PROJECT_DIR"
    PROJECT_DIRS+=("$PROJECT_DIR")

    log_step "Scenario: $SCENARIO"
    if [ "$SCENARIO_KIND" = "configurable" ]; then
        create_workspace
    fi
    write_legacy_config
    apply_legacy

    PHASE="snapshot-before"
    snapshot_products "$SCENARIO_DIR/products-before.json"
    assert_snapshot_settings "$SCENARIO_DIR/products-before.json"
    log_success "Captured live product configuration before migration"

    if [ "$SCENARIO" = "disabled" ]; then
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
    local selection="${1:-all}" product
    case "$selection" in
        list|--help|-h)
            print_usage
            return
            ;;
        all|simple|configurable|enabled|disabled)
            ;;
        simple:*)
            if ! is_simple_product "${selection#simple:}"; then
                log_error "Unknown simple product '${selection#simple:}'"
                print_usage
                return 2
            fi
            ;;
        *)
            log_error "Unknown scenario '$selection'"
            print_usage
            return 2
            ;;
    esac

    check_prerequisites
    build_binaries
    case "$selection" in
        all|simple)
            for product in "${SIMPLE_PRODUCTS[@]}"; do
                run_scenario "simple:$product"
            done
            if [ "$selection" = "all" ]; then
                run_scenario enabled
                run_scenario disabled
            fi
            ;;
        configurable)
            run_scenario enabled
            run_scenario disabled
            ;;
        *)
            run_scenario "$selection"
            ;;
    esac
    log_success "Product migration end-to-end tests passed ($selection)"
}

main "$@"
