#!/usr/bin/env bash
#
# Opt-in provider-only Compute state move. The target HCL is written manually:
# CDTOOL-1867 owns the Compute migration CLI workflow, not this release-drift test.
set -euo pipefail
umask 077
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
VERSION="${LEGACY_PROVIDER_VERSION:-10.3.0}"
PACKAGE="$REPO_ROOT/internal/acceptance_tests/fixtures/packages/valid.tar.gz"

[[ $# -eq 0 ]] || { echo "Usage: $0" >&2; exit 1; }
[[ -n "${FASTLY_API_TOKEN:-}" ]] || { echo "Set FASTLY_API_TOKEN for an isolated test account" >&2; exit 1; }
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Use an exact legacy provider version" >&2; exit 1; }
for cmd in terraform go jq; do
    command -v "$cmd" >/dev/null 2>&1 || { echo "Missing prerequisite: $cmd" >&2; exit 1; }
done
[[ -f "$PACKAGE" ]] || { echo "Missing repository Compute package fixture" >&2; exit 1; }

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-move-compute-schema.XXXXXX")"
PROJECT="$WORK_DIR/project"
PREFIX="tf-test-migcompute-$$-$(date +%s)"
PHASE="setup"
mkdir -p "$PROJECT" "$WORK_DIR/provider"

info() { printf '[INFO] %s\n' "$*"; }
fail() { printf '[ERROR] %s\n' "$*" >&2; exit 1; }

use_legacy() {
    export TF_CLI_CONFIG_FILE="$WORK_DIR/legacy.tfrc" FASTLY_API_KEY="$FASTLY_API_TOKEN"
}

use_target() {
    export TF_CLI_CONFIG_FILE="$WORK_DIR/target.tfrc"
    unset FASTLY_API_KEY
}

cleanup() {
    local status=$?
    trap - EXIT INT TERM
    unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH
    if (( status != 0 )); then
        printf '[ERROR] Failed in phase %s (exit %s)\n' "$PHASE" "$status" >&2
        if [[ -f "$PROJECT/terraform.tfstate" ]]; then
            if grep -Fq 'resource "fastly_service_compute_auto"' "$PROJECT/main.tf"; then
                use_target
            else
                use_legacy
            fi
            (cd "$PROJECT" && terraform destroy -auto-approve -input=false) ||
                info "Terraform cleanup failed; attempting prefix-scoped API cleanup"
        fi
        if [[ -f "$PROJECT/main.tf" ]]; then
            (cd "$REPO_ROOT" && go run ./scripts/cleanup-test-services -prefix "$PREFIX") ||
                info "API cleanup failed; check for services starting with $PREFIX"
        fi
    fi
    if [[ -n "${KEEP_WORK_DIR:-}" || $status -ne 0 ]]; then
        info "Private work directory retained: $WORK_DIR"
    else
        rm -rf -- "$WORK_DIR"
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

unset TF_CLI_ARGS TF_CLI_ARGS_init TF_CLI_ARGS_plan TF_CLI_ARGS_apply TF_CLI_ARGS_destroy
unset TF_DATA_DIR TF_WORKSPACE FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH

PHASE="build"
(cd "$REPO_ROOT" && go build -o "$WORK_DIR/provider/terraform-provider-fastly" .)
cat > "$WORK_DIR/legacy.tfrc" <<'EOF'
provider_installation {
  direct {}
}
EOF
cat > "$WORK_DIR/target.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "fastly/fastly" = "$WORK_DIR/provider"
  }
  direct {}
}
EOF
cp "$PACKAGE" "$PROJECT/package.tar.gz"

write_config() {
    local resource_type="$1" requirement="$2"
    cat > "$PROJECT/main.tf" <<EOF
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "$requirement"
    }
  }
}

provider "fastly" {}

resource "$resource_type" "demo" {
  name          = "$PREFIX"
  force_destroy = true

  domain {
    name = "$PREFIX.example.com"
  }

  backend {
    name    = "origin"
    address = "www.example.com"
    port    = 443
  }

  package {
    filename = "package.tar.gz"
  }
}
EOF
}

PHASE="legacy-apply"
cd "$PROJECT"
use_legacy
write_config fastly_service_compute "= $VERSION"
info "Creating a live Compute service with exact SDKv2 version $VERSION"
terraform init -backend=false -input=false
terraform version -json | jq -e --arg version "$VERSION" \
    '.provider_selections["registry.terraform.io/fastly/fastly"] == $version' >/dev/null ||
    fail "Terraform did not select exact SDKv2 version $VERSION"
terraform apply -auto-approve -input=false
terraform apply -refresh-only -auto-approve -input=false

# Verify the persisted schema, not just the provider lockfile.
terraform state pull | jq -e '
  [.resources[]? | select(.type == "fastly_service_compute" and .name == "demo")
   | .instances[]? | select(.schema_version == 0 and (.attributes.id | length) > 0)]
  | length == 1
' >/dev/null || fail "Legacy persisted Compute schema is not version 0"

# Snapshot the API hash and local-only input from actual state before moving.
terraform show -json | jq -e '
  .values.root_module.resources[] | select(.address == "fastly_service_compute.demo")
  | .values | {id, active_version, force_destroy, filename: .package[0].filename,
               hash: .package[0].source_code_hash}
  | select((.id | length) > 0 and .active_version > 0 and .force_destroy == true
           and .filename == "package.tar.gz" and (.hash | length) > 0)
' > "$WORK_DIR/before.json" || fail "Legacy package identity/hash missing"

PHASE="manual-state-move"
write_config fastly_service_compute_auto ">= 20.0.0"
cat > migration.tf <<'EOF'
moved {
  from = fastly_service_compute.demo
  to   = fastly_service_compute_auto.demo
}
EOF
use_target
rm -f .terraform.lock.hcl
terraform validate

PHASE="migration-plan"
TRACE="$WORK_DIR/provider-trace.log"
export FASTLY_DEBUG_MODE=true TF_LOG=TRACE TF_LOG_PATH="$TRACE"
terraform plan -input=false -out="$WORK_DIR/move.tfplan"
terraform show -json "$WORK_DIR/move.tfplan" > "$WORK_DIR/plan.json"
jq -e '
  all(.resource_changes[]?; all(.change.actions[]; . == "no-op" or . == "read"))
  and ([.resource_changes[]? | select(.address == "fastly_service_compute_auto.demo"
        and .previous_address == "fastly_service_compute.demo"
        and .change.importing == null)] | length == 1)
' "$WORK_DIR/plan.json" >/dev/null ||
    fail "Compute migration plan did not report a state-only move"

# Check the desired package input and actual hash BEFORE apply. A final no-op
# alone would not prove that the first apply avoided unnecessary uploads.
jq -e --slurpfile before "$WORK_DIR/before.json" '
  [.resource_changes[]? | select(.address == "fastly_service_compute_auto.demo")
   | select(.change.after.id == $before[0].id
     and .change.after.package[0].filename == $before[0].filename
     and .change.after.package[0].source_code_hash == $before[0].hash)]
  | length == 1
' "$WORK_DIR/plan.json" >/dev/null || fail "Migration plan lost Compute package identity/hash"

PHASE="migration-apply"
terraform apply -input=false "$WORK_DIR/move.tfplan"
code=0
terraform plan -input=false -detailed-exitcode || code=$?
[[ "$code" -eq 0 ]] || fail "Final Compute plan was not a no-op (exit $code)"
unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH

# Audit provider traffic for accidental service-version mutations/uploads.
calls="$(grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$TRACE" |
    sed -E 's/^.*"([A-Z]+ )/\1/' | sort -u || true)"
grep -q -E '^(GET|HEAD) /service/' <<< "$calls" || fail "Provider HTTP audit captured no service reads"
unsafe="$(awk '$1 !~ /^(GET|HEAD)$/ && $2 ~ /^\/(service|enabled-products)\// {print}' <<< "$calls")"
[[ -z "$unsafe" ]] || fail "Unexpected mutating service API requests: $unsafe"

PHASE="verify-state"
terraform state list > "$WORK_DIR/addresses.txt"
grep -Fxq 'fastly_service_compute_auto.demo' "$WORK_DIR/addresses.txt" ||
    fail "Target Compute state missing"
! grep -Fxq 'fastly_service_compute.demo' "$WORK_DIR/addresses.txt" ||
    fail "Legacy Compute state still present"
terraform show -json | jq -e --slurpfile before "$WORK_DIR/before.json" '
  [.values.root_module.resources[]? | select(.address == "fastly_service_compute_auto.demo")
   | .values | {id, active_version, force_destroy, filename: .package[0].filename,
                hash: .package[0].source_code_hash}]
  | length == 1 and .[0] == $before[0]
' >/dev/null || fail "Migration changed service identity, active version, or package"

PHASE="destroy"
terraform destroy -auto-approve -input=false
info "Compute state-only migration from SDKv2 $VERSION PASSED"
