#!/usr/bin/env bash

# End-to-end test for fastly-tf-migrate's configurable product migration
# (Bot Management, DDoS Protection, NGWAF) with non-default settings.
#
# Flow:
#   1. Create a legacy fastly_service_vcl with product_enablement using the
#      legacy fastly/fastly provider, then refresh state
#   2. fastly-tf-migrate analyze -> migrate --dry-run -> migrate --write
#   3. Switch to the locally built dual-model provider
#   4. terraform plan (must not create/delete/replace anything), apply
#   5. Final terraform plan must be a no-op (exit code 0)
#   6. Confirm the products are still enabled with the same settings
#   7. terraform destroy
#
# Requires FASTLY_API_TOKEN. A throwaway NGWAF workspace (mode "off") is created
# and deleted by the script unless FASTLY_TEST_NGWAF_WORKSPACE_ID is set, in
# which case that workspace is linked to the throwaway service and left alone.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-products.XXXXXX")"
PROJECT_DIR="$WORK_DIR/project"
PROVIDER_DIR="$WORK_DIR/provider"
BACKUP_DIR="$WORK_DIR/backups"
MIGRATOR="$WORK_DIR/fastly-tf-migrate"

SERVICE_NAME="tf-test-migprod-$$"
DOMAIN="$SERVICE_NAME.example.com"
LEGACY_PROVIDER_VERSION="${LEGACY_PROVIDER_VERSION:-9.5.0}"

CONTENTGUARD="on"
DDOS_MODE="block"
NGWAF_TRAFFIC_RAMP=25

PHASE="setup"
WORKSPACE_ID="${FASTLY_TEST_NGWAF_WORKSPACE_ID:-}"
CREATED_WORKSPACE=""
KEEP_WORK_DIR="${KEEP_WORK_DIR:-}"

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

cleanup() {
    local exit_code=$?
    trap - EXIT INT TERM

    if [ $exit_code -ne 0 ]; then
        log_error "Script failed in phase '$PHASE' with exit code $exit_code"
        log_step "Emergency cleanup"
        if [ -d "$PROJECT_DIR" ] && [ -f "$PROJECT_DIR/terraform.tfstate" ]; then
            (cd "$PROJECT_DIR" && terraform destroy -auto-approve -input=false) \
                || log_warning "terraform destroy failed; falling back to API sweep"
        fi
        (cd "$REPO_ROOT" && go run ./scripts/cleanup-test-services -prefix "$SERVICE_NAME") \
            || log_warning "API sweep failed; check for a leftover service named $SERVICE_NAME"
    fi

    # Runs on success too: the service is gone by now, so the workspace is unlinked.
    if [ -n "$CREATED_WORKSPACE" ]; then
        delete_workspace || log_warning "Could not delete workspace $CREATED_WORKSPACE; remove it manually"
    fi

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

create_workspace() {
    PHASE="workspace"
    if [ -n "$WORKSPACE_ID" ]; then
        log_info "Using existing NGWAF workspace from FASTLY_TEST_NGWAF_WORKSPACE_ID"
        return
    fi
    log_step "Creating throwaway NGWAF workspace"

    local response
    response="$(curl -s -w '\n%{http_code}' -X POST \
        -H "Fastly-Key: $FASTLY_API_TOKEN" -H "Content-Type: application/json" \
        -d "{\"name\":\"$SERVICE_NAME\",\"mode\":\"off\"}" \
        "https://api.fastly.com/ngwaf/v1/workspaces")"
    local status="${response##*$'\n'}" body="${response%$'\n'*}"
    if [ "$status" != "200" ] && [ "$status" != "201" ]; then
        log_error "Workspace creation failed (HTTP $status): $body"
        exit 1
    fi
    WORKSPACE_ID="$(echo "$body" | jq -r '.id')"
    if [ -z "$WORKSPACE_ID" ] || [ "$WORKSPACE_ID" = "null" ]; then
        log_error "Workspace response had no id: $body"
        exit 1
    fi
    CREATED_WORKSPACE="$WORKSPACE_ID"
    log_success "Created workspace $WORKSPACE_ID"
}

delete_workspace() {
    local status
    status="$(curl -s -o /dev/null -w '%{http_code}' -X DELETE \
        -H "Fastly-Key: $FASTLY_API_TOKEN" \
        "https://api.fastly.com/ngwaf/v1/workspaces/$CREATED_WORKSPACE")"
    case "$status" in
        200|202|204) log_info "Deleted workspace $CREATED_WORKSPACE" ;;
        *) return 1 ;;
    esac
}

build_binaries() {
    PHASE="build"
    log_step "Building provider and migrator"

    mkdir -p "$PROVIDER_DIR" "$PROJECT_DIR"
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
    name = "$DOMAIN"
  }

  backend {
    address = "example.com"
    name    = "origin"
    port    = 443
  }

  product_enablement {
    websockets = true

    bot_management {
      enabled      = true
      contentguard = "$CONTENTGUARD"
    }

    ddos_protection {
      enabled = true
      mode    = "$DDOS_MODE"
    }

    ngwaf {
      enabled      = true
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

run_migrator() {
    PHASE="migrate"
    log_step "Running fastly-tf-migrate"

    cd "$PROJECT_DIR"
    "$MIGRATOR" analyze .
    "$MIGRATOR" migrate --dry-run . | tee "$WORK_DIR/dry-run.diff"

    for expected in \
        "fastly_service_product_bot_management" \
        "fastly_service_product_ddos_protection" \
        "fastly_service_product_ngwaf"; do
        if ! grep -q "$expected" "$WORK_DIR/dry-run.diff"; then
            log_error "dry-run diff does not mention $expected"
            exit 1
        fi
    done

    "$MIGRATOR" migrate --write --backup-dir "$BACKUP_DIR" .

    if grep -q "product_enablement" main.tf; then
        log_error "legacy product_enablement block still present after migration"
        exit 1
    fi
    for expected in \
        "contentguard = \"$CONTENTGUARD\"" \
        "mode" "\"$DDOS_MODE\"" \
        "workspace_id = \"$WORKSPACE_ID\"" \
        "traffic_ramp = $NGWAF_TRAFFIC_RAMP"; do
        if ! grep -q -- "$expected" migration.tf; then
            log_error "migration.tf is missing: $expected"
            exit 1
        fi
    done
    log_success "Migration written; settings preserved in migration.tf"
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

    terraform plan -out="$WORK_DIR/migrate.plan" -input=false
    terraform show -json "$WORK_DIR/migrate.plan" > "$WORK_DIR/migrate.plan.json"

    local unsafe
    unsafe="$(jq -r '[.resource_changes[]?
        | select(.change.actions | index("delete") or index("create"))
        | "\(.address): \(.change.actions | join(","))"] | .[]' "$WORK_DIR/migrate.plan.json")"
    if [ -n "$unsafe" ]; then
        log_error "Plan proposes creating or deleting resources:"
        echo "$unsafe"
        exit 1
    fi

    local importing
    importing="$(jq '[.resource_changes[]? | select(.change.importing != null)] | length' "$WORK_DIR/migrate.plan.json")"
    if [ "$importing" -ne 4 ]; then
        log_error "Expected 4 product imports in the plan, found $importing"
        exit 1
    fi
    log_success "Plan imports 4 products and creates/deletes nothing"

    PHASE="ga-apply"
    terraform apply -input=false "$WORK_DIR/migrate.plan"

    PHASE="ga-noop-plan"
    log_step "Final plan must be a no-op"
    local rc=0
    terraform plan -detailed-exitcode -input=false || rc=$?
    if [ $rc -ne 0 ]; then
        log_error "Final plan exit code $rc (0 = no changes, 2 = changes)"
        exit 1
    fi
    log_success "Final plan is a no-op"
}

verify_products() {
    PHASE="verify-api"
    log_step "Verifying products against the API"

    local product status
    for product in bot_management ddos_protection ngwaf; do
        status="$(curl -s -o /dev/null -w '%{http_code}' \
            -H "Fastly-Key: $FASTLY_API_TOKEN" \
            "https://api.fastly.com/enabled-products/v1/$product/services/$SERVICE_ID")"
        if [ "$status" != "200" ]; then
            log_error "$product is not enabled on $SERVICE_ID (HTTP $status)"
            exit 1
        fi
        log_success "$product is enabled"
    done
}

destroy() {
    PHASE="destroy"
    log_step "Destroying resources"

    cd "$PROJECT_DIR"
    terraform destroy -auto-approve -input=false
    log_success "Destroyed"
}

main() {
    check_prerequisites
    create_workspace
    build_binaries
    write_legacy_config
    apply_legacy
    run_migrator
    plan_and_apply_ga
    verify_products
    destroy
    log_success "Configurable product migration end-to-end test passed"
}

main "$@"
