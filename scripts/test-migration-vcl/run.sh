#!/usr/bin/env bash
#
# Real Fastly/Terraform acceptance test for the legacy VCL -> CDN Auto state move.
# Deliberately does NOT enable or import any Fastly products.
# Scope: a VCL service with domain and backend, without any product enablement.
# The opt-in logging mode adds trusted test-owned endpoints to the same service.
#
# This is deliberately opt-in: it creates and destroys LIVE Fastly services.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-vcl.XXXXXX")"
PROVIDER_DIR="$WORK_DIR/provider"
MIGRATOR="$WORK_DIR/fastly-tf-migrate"
BACKUP_ROOT="$WORK_DIR/backups"
NAME_PREFIX="tf-test-migvcl-$$-$(date +%s)"
SERVICE_NAME="$NAME_PREFIX-svc"
API_BASE="${FASTLY_API_URL:-https://api.fastly.com}"
LEGACY_PROVIDER_VERSION="${LEGACY_PROVIDER_VERSION:-10.3.0}"
KEEP_WORK_DIR="${KEEP_WORK_DIR:-}"
LOGGING_TLS="${MIGRATION_TEST_LOGGING_TLS:-false}"
LOGGING_FIXTURE="${MIGRATION_LOGGING_FIXTURE:-}"
LOGGING_SUMMARY=""
[[ "$LOGGING_TLS" == true || "$LOGGING_TLS" == false ]] || { echo "MIGRATION_TEST_LOGGING_TLS must be true or false" >&2; exit 1; }
if [[ "$LOGGING_TLS" == true ]]; then
    # The legacy provider may display fields such as HTTPS header_value in
    # plaintext. Do not stream any Terraform output, logs, diffs or traces.
    exec 3>&1
    exec >"$WORK_DIR/live.log" 2>&1
    printf '[INFO] Logging migration acceptance: raw logs and state remain private.\n' >&3
fi

PHASE="setup"
PROJECT_DIR="$WORK_DIR/project"
SERVICE_ID=""
LEGACY_LIFECYCLE_JSON=""

info() { printf '[INFO] %s\n' "$*"; }
fail() {
    printf '[ERROR] %s\n' "$*" >&2
    # Prerequisite diagnostics contain only fixed messages and tool names.
    # Later phases may involve sensitive receiver configuration: keep them private.
    if [[ "$LOGGING_TLS" == true && "$PHASE" == prerequisites ]]; then
        printf '[ERROR] %s\n' "$*" >&3
    fi
    exit 1
}

use_legacy_provider() {
    unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH
    export TF_CLI_CONFIG_FILE="$WORK_DIR/legacy.tfrc"
    export FASTLY_API_KEY="$FASTLY_API_TOKEN"
}

use_new_provider() {
    unset FASTLY_API_KEY
    export TF_CLI_CONFIG_FILE="$WORK_DIR/.terraformrc"
}

cleanup() {
    local status=$?
    trap - EXIT INT TERM
    unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH

    if (( status != 0 )); then
        printf '[ERROR] Failed in phase %s (exit %s)\n' \
            "$PHASE" "$status" >&2
        if [[ -f "$PROJECT_DIR/terraform.tfstate" ]]; then
            if grep -Fq 'resource "fastly_service_cdn_auto"' "$PROJECT_DIR/main.tf"; then
                use_new_provider
            else
                use_legacy_provider
            fi
            (cd "$PROJECT_DIR" && terraform destroy -auto-approve -input=false) ||
                info "Terraform destroy failed; attempting API cleanup"
        fi
        # Only sweep after test-project setup began, and only our unique prefix.
        if [[ -d "$PROJECT_DIR" ]]; then
            (cd "$REPO_ROOT" && go run ./scripts/cleanup-test-services -prefix "$NAME_PREFIX") ||
                info "API cleanup failed; check for services starting with $NAME_PREFIX"
        fi
    fi

    if [[ "$LOGGING_TLS" == true ]]; then
        if (( status == 0 )); then
            printf '[INFO] Logging endpoint types/counts: %s\n' "$LOGGING_SUMMARY" >&3
            printf '[INFO] Exact persisted endpoint config/credentials, service ID and active version preserved; zero mutating service requests\n' >&3
            printf '[INFO] CDN logging TLS/auth migration from SDKv2 %s PASSED (including cleanup)\n' "$LEGACY_PROVIDER_VERSION" >&3
        else
            printf '[ERROR] Logging migration failed in phase %s (exit %s). Inspect the private log locally.\n' "$PHASE" "$status" >&3
        fi
        if [[ -n "$KEEP_WORK_DIR" || $status -ne 0 ]]; then
            printf '[INFO] Private workspace: %s (do not share state, plans or live.log)\n' "$WORK_DIR" >&3
        fi
    fi
    if [[ -n "$KEEP_WORK_DIR" || $status -ne 0 ]]; then
        info "Work directory retained: $WORK_DIR (contains Terraform state; keep private)"
    else
        rm -rf -- "$WORK_DIR"
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

check_prerequisites() {
    PHASE="prerequisites"
    [[ -n "${FASTLY_API_TOKEN:-}" ]] || fail "Set FASTLY_API_TOKEN for an isolated Fastly test account"
    [[ "$API_BASE" == https://* ]] || fail "FASTLY_API_URL must be an HTTPS URL"
    [[ "$LEGACY_PROVIDER_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
        fail "LEGACY_PROVIDER_VERSION must be an exact published release (for example 10.3.0)"
    local cmd
    for cmd in terraform go jq curl cksum cmp; do
        command -v "$cmd" >/dev/null 2>&1 || fail "Missing prerequisite: $cmd"
    done
    if [[ "$LOGGING_TLS" == true ]]; then
        [[ "$LOGGING_FIXTURE" == /* && -r "$LOGGING_FIXTURE" && -s "$LOGGING_FIXTURE" ]] ||
            fail "Set MIGRATION_LOGGING_FIXTURE to an absolute, readable, nonempty file containing only trusted test logging blocks"
    fi
    info "Running against $API_BASE; this test creates and deletes real Fastly resources"
}

build_binaries() {
    PHASE="build"
    mkdir -p "$PROVIDER_DIR"
    (
        cd "$REPO_ROOT"
        # Match #231's pre-GA local-provider setup. The repository's beta
        # binary will become fastly/fastly when the dual-model provider goes GA.
        go build -o "$PROVIDER_DIR/terraform-provider-fastly" .
        go build -o "$MIGRATOR" ./cmd/fastly-tf-migrate
    )
    cat > "$WORK_DIR/legacy.tfrc" <<'EOF'
provider_installation {
  direct {}
}
EOF
    cat > "$WORK_DIR/.terraformrc" <<EOF
provider_installation {
  dev_overrides {
    "fastly/fastly" = "$PROVIDER_DIR"
  }
  direct {}
}
EOF
}

write_legacy_config() {
    PHASE="legacy-config"
    cat > "$PROJECT_DIR/main.tf" <<EOF
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "= $LEGACY_PROVIDER_VERSION"
    }
  }
}

provider "fastly" {}

resource "fastly_service_vcl" "demo" {
  name          = "$SERVICE_NAME"
  force_destroy = true

  domain {
    name    = "$SERVICE_NAME.example.com"
    comment = "VCL migration E2E test"
  }

  backend {
    name    = "origin"
    address = "www.example.com"
    port    = 443
  }
}
EOF
    if [[ "$LOGGING_TLS" == true ]]; then
        # Keep the same isolated service, with the supplied logging blocks
        # inside it. Do not use a fixture from an untrusted source.
        sed '$d' "$PROJECT_DIR/main.tf" > "$PROJECT_DIR/main.tf.tmp"
        cat >> "$PROJECT_DIR/main.tf.tmp" <<'HCL'

  condition {
    name = "migration-logging-response"
    type = "RESPONSE"
    statement = "resp.status == 200"
    priority = 10
  }
HCL
        cat "$LOGGING_FIXTURE" >> "$PROJECT_DIR/main.tf.tmp"
        printf '\n}\n' >> "$PROJECT_DIR/main.tf.tmp"
        mv "$PROJECT_DIR/main.tf.tmp" "$PROJECT_DIR/main.tf"
    fi
}

snapshot_logging_state() {
    local type="$1" destination="$2"
    terraform state pull | jq -eS --arg type "$type" \
        -f "$REPO_ROOT/scripts/test-migration-logging-tls/snapshot.jq" > "$destination" ||
        fail "Could not capture a complete private logging state snapshot"
}

apply_legacy() {
    PHASE="legacy-apply"
    cd "$PROJECT_DIR"
    use_legacy_provider
    terraform init -input=false
    terraform version -json | jq -e --arg version "$LEGACY_PROVIDER_VERSION" '
      .provider_selections["registry.terraform.io/fastly/fastly"] == $version
    ' >/dev/null || fail "Terraform did not select pinned legacy provider $LEGACY_PROVIDER_VERSION"
    info "Selected exact SDKv2 version $LEGACY_PROVIDER_VERSION"
    if [[ "$LOGGING_TLS" == true ]]; then
        terraform plan -input=false -out="$WORK_DIR/legacy.tfplan"
        terraform show -json "$WORK_DIR/legacy.tfplan" | jq -e '
          [.resource_changes[]? | select(.mode == "managed") |
            {address, actions:.change.actions}] ==
          [{address:"fastly_service_vcl.demo", actions:["create"]}]
        ' >/dev/null || fail "Logging fixture must create only the isolated test service"
        terraform apply -input=false "$WORK_DIR/legacy.tfplan"
    else
        terraform apply -auto-approve -input=false
    fi
    # The supported SDKv2 persisted schema is 1, not merely provider >= 9.1.
    terraform apply -refresh-only -auto-approve -input=false

    SERVICE_ID="$(terraform show -json |
        jq -r '.values.root_module.resources[]?
          | select(.address == "fastly_service_vcl.demo") | .values.id')"
    [[ -n "$SERVICE_ID" && "$SERVICE_ID" != null ]] ||
        fail "Legacy service ID missing from Terraform state"

    # `reuse` cannot be set together with `force_destroy`, even when false.
    # Snapshot the actual legacy lifecycle values; only assert reuse after the
    # move if it was present (non-null) in the source state.
    LEGACY_LIFECYCLE_JSON="$(terraform show -json | jq -c --arg id "$SERVICE_ID" '
      .values.root_module.resources[]?
      | select(.address == "fastly_service_vcl.demo" and .values.id == $id)
      | {force_destroy: .values.force_destroy, reuse: .values.reuse}
    ')"
    [[ -n "$LEGACY_LIFECYCLE_JSON" ]] ||
        fail "Legacy lifecycle attributes missing from Terraform state"
    jq -e '.force_destroy == true' <<< "$LEGACY_LIFECYCLE_JSON" >/dev/null ||
        fail "Expected legacy force_destroy=true in Terraform state"

    # Check persisted schema separately from the lockfile version.
    terraform state pull > "$WORK_DIR/legacy-state-check.json"
    jq -e --arg id "$SERVICE_ID" '
      [.resources[]? | select(.type == "fastly_service_vcl" and .name == "demo")
       | .instances[]? | select(.schema_version == 1 and .attributes.id == $id)]
      | length == 1
    ' "$WORK_DIR/legacy-state-check.json" >/dev/null ||
        fail "Legacy persisted VCL state must have schema version 1 and the same service ID"
    # The state check file may include secrets; do not keep a redundant copy.
    rm -f "$WORK_DIR/legacy-state-check.json"
    info "Legacy VCL service $SERVICE_ID has persisted schema 1"
    if [[ "$LOGGING_TLS" == true ]]; then
        snapshot_logging_state fastly_service_vcl "$WORK_DIR/logging-before.json"
        LOGGING_SUMMARY="$(jq -r '.logging | to_entries | map(select(.value|length>0) | "\(.key)=\(.value|length)") | join(", ")' "$WORK_DIR/logging-before.json")"
    fi
}

snapshot_live_service() {
    local destination="$1"
    local body
    body="$(curl --fail --silent --show-error \
        -H "Fastly-Key: $FASTLY_API_TOKEN" "$API_BASE/service/$SERVICE_ID")"
    printf '%s\n' "$body" | jq -e --arg id "$SERVICE_ID" \
        --arg name "$SERVICE_NAME" '.id == $id and .name == $name' >/dev/null ||
        fail "Live Fastly service identity does not match $SERVICE_ID / $SERVICE_NAME"
    # Ignore volatile API metadata; a migration must not change the identity,
    # name, or active service version.
    printf '%s\n' "$body" |
        jq -S '{id, name, active_version: (.active_version // null)}' > "$destination"
}

run_migrator() {
    PHASE="migrate"
    cd "$PROJECT_DIR"
    "$MIGRATOR" analyze . | tee "$WORK_DIR/analyze-before.txt"
    local before
    before="$(cksum main.tf)"
    "$MIGRATOR" migrate --dry-run . | tee "$WORK_DIR/dry-run.txt"
    grep -Fxq 'Migration dry-run: READY' "$WORK_DIR/dry-run.txt" ||
        fail "Migration dry-run was not READY"
    grep -Fxq 'Resources to move: 1' "$WORK_DIR/dry-run.txt" ||
        fail "Expected one VCL service move"
    [[ "$before" == "$(cksum main.tf)" && ! -e migration.tf ]] ||
        fail "Dry-run modified Terraform configuration"

    if [[ "$LOGGING_TLS" == true ]]; then
        grep -Fq 'HCL diff withheld' "$WORK_DIR/dry-run.txt" ||
            fail "Sensitive logging dry-run did not withhold the source diff"
        "$MIGRATOR" migrate --write --backup-dir "$BACKUP_ROOT" . | tee "$WORK_DIR/write-preview.txt"
        grep -Fq 'HCL diff withheld' "$WORK_DIR/write-preview.txt" ||
            fail "Sensitive logging write preview did not withhold the source diff"
    else
        "$MIGRATOR" migrate --write --backup-dir "$BACKUP_ROOT" .
    fi
    grep -Fq 'resource "fastly_service_cdn_auto" "demo"' main.tf ||
        fail "Main resource was not rewritten to fastly_service_cdn_auto"
    ! grep -Fq 'resource "fastly_service_vcl"' main.tf ||
        fail "Legacy VCL resource still present"
    grep -Eq 'from[[:space:]]*=[[:space:]]*fastly_service_vcl\.demo' migration.tf ||
        fail "Missing moved.from in migration.tf"
    grep -Eq 'to[[:space:]]*=[[:space:]]*fastly_service_cdn_auto\.demo' migration.tf ||
        fail "Missing moved.to in migration.tf"
    grep -Fq 'version = ">= 20.0.0"' main.tf ||
        fail "Migrated provider requirement must use GA >= 20.0.0"

    ! grep -Fq 'product_enablement' main.tf ||
        fail "VCL fixture unexpectedly includes product enablement"
    ! grep -Fq 'fastly_service_product_' migration.tf ||
        fail "VCL-only migration generated a product resource"
    ! grep -Eq '^[[:space:]]*import[[:space:]]*\{' migration.tf ||
        fail "VCL-only migration unexpectedly generated an import block"

    # --write must produce a recoverable private HCL backup.
    [[ -n "$(find "$BACKUP_ROOT" -type f -name manifest.json -print -quit)" ]] ||
        fail "Migration recovery manifest was not created"
    info "Wrote migrated HCL with a recovery backup"
}

# Parse go-fastly HTTP dumps from the Terraform provider's debug trace.
provider_calls() {
    grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$1" |
        sed -E 's/^.*"([A-Z]+ )/\1/' | sort -u || true
}

assert_no_mutating_service_calls() {
    local trace="$1" calls unsafe
    calls="$(provider_calls "$trace")"
    if ! grep -q -E '^(GET|HEAD) /service/' <<< "$calls"; then
        fail "No service GET/HEAD captured; provider HTTP audit was not active"
    fi
    unsafe="$(awk '$1 !~ /^(GET|HEAD)$/ && $2 ~ /^\/(service|enabled-products)\// {print}' <<< "$calls")"
    [[ -z "$unsafe" ]] ||
        fail "Provider made mutating service/product API requests during migration: $unsafe"
    info "Observed service refresh calls but no mutating service/product API requests"
}

plan_apply_and_verify() {
    PHASE="new-plan"
    cd "$PROJECT_DIR"
    use_new_provider
    # Pre-GA dev_override, consistent with the existing product test: no 20.x
    # registry package yet. The stale *test-project* lockfile can be dropped.
    rm -f .terraform.lock.hcl
    terraform validate

    local trace="$WORK_DIR/migration-provider-trace.log"
    : > "$trace"
    export FASTLY_DEBUG_MODE=true TF_LOG=TRACE TF_LOG_PATH="$trace"

    terraform plan -input=false -out="$WORK_DIR/migration.tfplan"
    terraform show -json "$WORK_DIR/migration.tfplan" > "$WORK_DIR/migration-plan.json"

    local unsafe
    unsafe="$(jq -r '
      .resource_changes[]?
      | select((.change.actions | all(. == "no-op" or . == "read")) | not)
      | "\(.address): \(.change.actions | join(","))"
    ' "$WORK_DIR/migration-plan.json")"
    [[ -z "$unsafe" ]] ||
        fail "Migration plan contains create/update/delete/replace actions: $unsafe"

    jq -e '
      [.resource_changes[]?
       | select(.address == "fastly_service_cdn_auto.demo"
          and .previous_address == "fastly_service_vcl.demo")]
      | length == 1
    ' "$WORK_DIR/migration-plan.json" >/dev/null ||
        fail "Terraform plan did not identify the legacy VCL -> CDN Auto move"

    local actual_imports
    actual_imports="$(jq '[.resource_changes[]?
      | select(.change.importing != null)] | length' "$WORK_DIR/migration-plan.json")"
    [[ "$actual_imports" == 0 ]] ||
        fail "VCL-only migration must have zero imports; got $actual_imports"
    info "Migration plan: VCL state move, zero imports, no infrastructure changes"

    PHASE="new-apply"
    terraform apply -input=false "$WORK_DIR/migration.tfplan"

    PHASE="new-no-op"
    local code=0
    terraform plan -detailed-exitcode -input=false || code=$?
    [[ "$code" -eq 0 ]] ||
        fail "Final Terraform plan was not a no-op (exit code $code)"

    unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH
    assert_no_mutating_service_calls "$trace"
    terraform state list > "$WORK_DIR/new-state-addresses.txt"
    grep -Fxq 'fastly_service_cdn_auto.demo' "$WORK_DIR/new-state-addresses.txt" ||
        fail "Target CDN Auto service missing from Terraform state"
    ! grep -Fxq 'fastly_service_vcl.demo' "$WORK_DIR/new-state-addresses.txt" ||
        fail "Legacy VCL state address still present"
    ! grep -Fq 'fastly_service_product_' "$WORK_DIR/new-state-addresses.txt" ||
        fail "VCL-only migration unexpectedly contains product state"

    terraform show -json | jq -e --arg id "$SERVICE_ID" \
      --argjson legacy "$LEGACY_LIFECYCLE_JSON" '
      [.values.root_module.resources[]?
       | select(.address == "fastly_service_cdn_auto.demo"
           and .values.id == $id
           and .values.force_destroy == $legacy.force_destroy
           and ($legacy.reuse == null or .values.reuse == $legacy.reuse))]
      | length == 1
    ' >/dev/null || fail "CDN Auto state lost service ID or legacy lifecycle settings"

    if [[ "$LOGGING_TLS" == true ]]; then
        snapshot_logging_state fastly_service_cdn_auto "$WORK_DIR/logging-after.json"
        cmp -s "$WORK_DIR/logging-before.json" "$WORK_DIR/logging-after.json" ||
            fail "Logging state changed across migration (values withheld)"
    fi

    "$MIGRATOR" analyze . > "$WORK_DIR/analyze-after.txt"
    "$MIGRATOR" migrate --dry-run . > "$WORK_DIR/dry-run-after.txt"
    grep -Fxq 'Resources to move: 0' "$WORK_DIR/dry-run-after.txt" ||
        fail "Repeated migration still proposes a state move"
    grep -Fxq 'Files to change: 0' "$WORK_DIR/dry-run-after.txt" ||
        fail "Repeated migration is not idempotent"
}

main() {
    [[ $# -eq 0 ]] || fail "Usage: $0"
    check_prerequisites
    unset TF_DATA_DIR TF_WORKSPACE TF_CLI_ARGS TF_CLI_ARGS_init TF_CLI_ARGS_plan TF_CLI_ARGS_apply TF_CLI_ARGS_destroy
    build_binaries

    mkdir -p "$PROJECT_DIR"
    info "Starting VCL -> CDN Auto state-migration acceptance test"
    write_legacy_config
    apply_legacy
    snapshot_live_service "$WORK_DIR/service-before.json"

    run_migrator
    plan_apply_and_verify

    PHASE="verify-live"
    snapshot_live_service "$WORK_DIR/service-after.json"
    diff -u "$WORK_DIR/service-before.json" "$WORK_DIR/service-after.json" ||
        fail "Live Fastly service identity/name/active version changed during migration"

    PHASE="destroy"
    cd "$PROJECT_DIR"
    terraform destroy -auto-approve -input=false
    info "VCL -> CDN Auto state-migration acceptance test PASSED"
}

main "$@"
