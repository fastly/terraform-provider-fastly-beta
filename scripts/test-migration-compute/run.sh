#!/usr/bin/env bash

# End-to-end test for the fastly_service_compute -> fastly_service_compute_auto
# migration (fastly-tf-migrate plus the provider's MoveState).
#
#   1. The migrator must refuse activate = false before writing anything
#   2. Create a legacy fastly_service_compute (package, domain, backend) with the
#      legacy fastly/fastly provider, then refresh state (schema v0)
#   3. fastly-tf-migrate analyze -> migrate --dry-run -> migrate --write
#   4. Switch to the locally built dual-model provider
#   5. terraform plan: the service must be moved, never created or deleted
#      (so never replaced), and the plan must record the previous address
#   6. terraform apply, then a final plan that must be a no-op (exit code 0)
#   7. The Fastly service ID must be unchanged
#   8. terraform destroy
#
# Requires FASTLY_API_TOKEN. Creates a real service; it is destroyed at the end,
# or swept by name prefix if the run fails.

set -euo pipefail
umask 077

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-compute.XXXXXX")"
PROVIDER_DIR="$WORK_DIR/provider"
BACKUP_DIR="$WORK_DIR/backups"
MIGRATOR="$WORK_DIR/fastly-tf-migrate"
PACKAGE="$REPO_ROOT/internal/acceptance_tests/fixtures/packages/valid.tar.gz"

NAME_PREFIX="tf-test-migcompute-$$"
SERVICE_NAME="$NAME_PREFIX-svc"
LEGACY_PROVIDER_VERSION="${LEGACY_PROVIDER_VERSION:-9.5.0}"
PROJECT_DIR="$WORK_DIR/project"
KEEP_WORK_DIR="${KEEP_WORK_DIR:-}"
PHASE="setup"
LEGACY_ID=""
LEGACY_LIFECYCLE_JSON=""

log_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
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
        if [ -f "$PROJECT_DIR/terraform.tfstate" ]; then
            (cd "$PROJECT_DIR" && terraform destroy -auto-approve -input=false) \
                || log_error "terraform destroy failed; falling back to API sweep"
        fi
        (cd "$REPO_ROOT" && go run ./scripts/cleanup-test-services -prefix "$NAME_PREFIX") \
            || log_error "API sweep failed; check for leftover services named $NAME_PREFIX*"
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
    if [ -z "${FASTLY_API_TOKEN:-}" ]; then
        log_error "FASTLY_API_TOKEN is not set"
        exit 1
    fi
    local cmd
    for cmd in terraform go jq; do
        if ! command -v "$cmd" > /dev/null 2>&1; then
            log_error "Required command '$cmd' not found"
            exit 1
        fi
    done
    if [ ! -f "$PACKAGE" ]; then
        log_error "Compute package fixture not found: $PACKAGE"
        exit 1
    fi
}

build_binaries() {
    PHASE="build"
    log_step "Building provider and migrator"
    mkdir -p "$PROVIDER_DIR" "$PROJECT_DIR"
    cd "$REPO_ROOT"
    # The migrator keeps source = "fastly/fastly", so the local build is served
    # under that address.
    go build -o "$PROVIDER_DIR/terraform-provider-fastly"
    go build -o "$MIGRATOR" ./cmd/fastly-tf-migrate

    cat > "$WORK_DIR/.terraformrc" << RC
provider_installation {
  dev_overrides {
    "fastly/fastly" = "$PROVIDER_DIR"
  }
  direct {}
}
RC
}

write_legacy_config() {
    cp "$PACKAGE" "$PROJECT_DIR/package.tar.gz"
    cat > "$PROJECT_DIR/main.tf" << HCL
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = ">= $LEGACY_PROVIDER_VERSION, < 20.0.0"
    }
  }
}

provider "fastly" {}

# Fastly's package hash is not a plain file digest, so filesha512() would show a
# permanent diff against the hash the API reports.
data "fastly_package_hash" "pkg" {
  filename = "package.tar.gz"
}

resource "fastly_service_compute" "demo" {
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

  package {
    filename         = "package.tar.gz"
    source_code_hash = data.fastly_package_hash.pkg.hash
  }
}
HCL
}

service_id() {
    terraform show -json | jq -r --arg a "$1" \
        '.values.root_module.resources[] | select(.address==$a) | .values.id'
}

apply_legacy() {
    PHASE="legacy-apply"
    log_step "Applying legacy configuration"
    cd "$PROJECT_DIR"
    use_legacy_provider
    terraform init -input=false
    terraform apply -auto-approve -input=false
    # Persists state at the supported legacy schema version.
    terraform apply -refresh-only -auto-approve -input=false
    LEGACY_ID="$(service_id fastly_service_compute.demo)"
    if [ -z "$LEGACY_ID" ] || [ "$LEGACY_ID" = "null" ]; then
        log_error "Could not read the legacy service ID from state"
        exit 1
    fi
    LEGACY_LIFECYCLE_JSON="$(terraform show -json | jq -c --arg id "$LEGACY_ID" '
        .values.root_module.resources[]?
        | select(.address == "fastly_service_compute.demo" and .values.id == $id)
        | {force_destroy: .values.force_destroy, reuse: .values.reuse}')"
    [ -n "$LEGACY_LIFECYCLE_JSON" ] || { log_error "Legacy lifecycle attributes missing from state"; exit 1; }
    terraform state pull | jq -e --arg id "$LEGACY_ID" '
        [.resources[]? | select(.type == "fastly_service_compute" and .name == "demo")
         | .instances[]? | select(.schema_version == 0 and .attributes.id == $id)]
        | length == 1' > /dev/null \
        || { log_error "Legacy persisted Compute state must have schema version 0"; exit 1; }
    log_success "Legacy service created: $LEGACY_ID (persisted schema 0)"
}

run_migrator() {
    PHASE="migrate"
    log_step "Running fastly-tf-migrate"
    cd "$PROJECT_DIR"
    "$MIGRATOR" analyze .
    local before
    before="$(cksum main.tf)"
    "$MIGRATOR" migrate --dry-run . | tee "$WORK_DIR/dry-run.diff"
    grep -Fxq 'Migration dry-run: READY' "$WORK_DIR/dry-run.diff" \
        || { log_error "Migration dry-run was not READY"; exit 1; }
    grep -Fxq 'Resources to move: 1' "$WORK_DIR/dry-run.diff" \
        || { log_error "Expected one Compute service move"; exit 1; }
    if [ "$before" != "$(cksum main.tf)" ] || [ -e migration.tf ]; then
        log_error "Dry-run modified Terraform configuration"
        exit 1
    fi
    if ! grep -q "fastly_service_compute_auto" "$WORK_DIR/dry-run.diff"; then
        log_error "dry-run diff does not mention fastly_service_compute_auto"
        exit 1
    fi
    "$MIGRATOR" migrate --write --backup-dir "$BACKUP_DIR" .
    if ! grep -q 'resource "fastly_service_compute_auto" "demo"' main.tf; then
        log_error "main.tf was not rewritten to fastly_service_compute_auto"
        exit 1
    fi
    if grep -Fq '< 20.0.0' main.tf migration.tf 2> /dev/null; then
        log_error "Provider constraint was not raised to the GA provider"
        exit 1
    fi
    if ! grep -q "fastly_service_compute.demo" migration.tf; then
        log_error "migration.tf has no moved block"
        exit 1
    fi
    if [ -z "$(find "$BACKUP_DIR" -type f -name manifest.json -print -quit)" ]; then
        log_error "Migration recovery manifest was not created"
        exit 1
    fi
    if grep -Fq 'fastly_service_product_' migration.tf || grep -Eq '^[[:space:]]*import[[:space:]]*\{' migration.tf; then
        log_error "Product-free Compute migration generated product resources or imports"
        exit 1
    fi
    log_success "Migration written"
}

plan_and_apply_ga() {
    PHASE="ga-plan"
    log_step "Switching to the dual-model provider"
    cd "$PROJECT_DIR"
    use_ga_provider
    # The migrator raises the constraint to >= 20.0.0, which init would try to resolve from the
    # registry, and no 20.x release exists yet. The local build is served via dev_overrides, so
    # skip init and drop the stale legacy lock entry. Re-add init once 20.x is published.
    rm -f .terraform.lock.hcl
    terraform validate

    terraform plan -out="$WORK_DIR/migrate.plan" -input=false
    terraform show -json "$WORK_DIR/migrate.plan" > "$WORK_DIR/migrate.plan.json"

    local unsafe
    unsafe="$(jq -r '[.resource_changes[]?
        | select(.change.actions | any(. == "create" or . == "delete"))
        | "\(.address): \(.change.actions | join(","))"] | .[]' "$WORK_DIR/migrate.plan.json")"
    if [ -n "$unsafe" ]; then
        log_error "Plan recreates resources instead of moving them:"
        echo "$unsafe"
        exit 1
    fi
    local previous
    previous="$(jq -r '.resource_changes[]? | select(.address=="fastly_service_compute_auto.demo") | .previous_address // empty' "$WORK_DIR/migrate.plan.json")"
    if [ "$previous" != "fastly_service_compute.demo" ]; then
        log_error "Plan did not move fastly_service_compute.demo (previous_address='$previous')"
        exit 1
    fi
    log_success "Plan moves the service without create, delete or replace"

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

    local new_id
    new_id="$(service_id fastly_service_compute_auto.demo)"
    if [ "$new_id" != "$LEGACY_ID" ]; then
        log_error "Service ID changed: $LEGACY_ID -> $new_id"
        exit 1
    fi
    log_success "Service ID preserved: $new_id"

    terraform state list > "$WORK_DIR/new-state-addresses.txt"
    if ! grep -Fxq 'fastly_service_compute_auto.demo' "$WORK_DIR/new-state-addresses.txt" \
        || grep -Fxq 'fastly_service_compute.demo' "$WORK_DIR/new-state-addresses.txt"; then
        log_error "State must contain only the new Compute Auto address"
        exit 1
    fi
    terraform show -json | jq -e --arg id "$LEGACY_ID" --argjson legacy "$LEGACY_LIFECYCLE_JSON" '
        [.values.root_module.resources[]?
         | select(.address == "fastly_service_compute_auto.demo"
             and .values.id == $id
             and .values.force_destroy == $legacy.force_destroy
             and ($legacy.reuse == null or .values.reuse == $legacy.reuse))]
        | length == 1' > /dev/null \
        || { log_error "Compute Auto state lost service ID or legacy lifecycle settings"; exit 1; }

    "$MIGRATOR" migrate --dry-run . > "$WORK_DIR/dry-run-after.txt"
    grep -Fxq 'Resources to move: 0' "$WORK_DIR/dry-run-after.txt" \
        || { log_error "Repeated migration still proposes a state move"; exit 1; }
    grep -Fxq 'Files to change: 0' "$WORK_DIR/dry-run-after.txt" \
        || { log_error "Repeated migration is not idempotent"; exit 1; }
    log_success "Migration is idempotent on rerun"
}

expect_activate_false_blocked() {
    PHASE="migrate-blocked"
    log_step "Migrator must refuse activate = false"
    local blocked="$WORK_DIR/blocked"
    mkdir -p "$blocked"
    cat > "$blocked/main.tf" << HCL
resource "fastly_service_compute" "demo" {
  name     = "$SERVICE_NAME"
  activate = false
}
HCL
    local before after
    before="$(cksum "$blocked/main.tf")"
    if "$MIGRATOR" migrate --write --backup-dir "$WORK_DIR/blocked-backups" "$blocked" > "$WORK_DIR/blocked.out" 2>&1; then
        log_error "migrate --write accepted activate = false"
        cat "$WORK_DIR/blocked.out"
        exit 1
    fi
    after="$(cksum "$blocked/main.tf")"
    if [ "$before" != "$after" ] || [ -e "$blocked/migration.tf" ]; then
        log_error "A blocked migration changed the configuration"
        exit 1
    fi
    log_success "activate = false was refused before any write"
}

main() {
    check_prerequisites
    build_binaries
    expect_activate_false_blocked
    write_legacy_config
    apply_legacy
    run_migrator
    plan_and_apply_ga

    PHASE="destroy"
    log_step "Destroying resources"
    cd "$PROJECT_DIR"
    terraform destroy -auto-approve -input=false
    log_success "Compute migration end-to-end test passed"
}

main "$@"
