#!/usr/bin/env bash
# Opt-in live test for the fastly_compute_acl -> fastly_acl state move.
# Creates and deletes one Compute ACL. Requires FASTLY_API_TOKEN for an
# isolated test account.
#
# The rename and moved block are written by hand: this tests the provider's
# state move, not the migration CLI. An entry added outside Terraform must be
# left untouched by the move.
set -euo pipefail
umask 077

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LEGACY_PROVIDER_VERSION:-10.3.0}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Use an exact legacy release" >&2; exit 1; }
: "${FASTLY_API_TOKEN:?Set FASTLY_API_TOKEN for an isolated test account}"
for cmd in go terraform jq curl; do
    command -v "$cmd" >/dev/null || { echo "Missing command: $cmd" >&2; exit 1; }
done
API_BASE="${FASTLY_API_URL:-https://api.fastly.com}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-compute-acl.XXXXXX")"
PROJECT="$WORK_DIR/project"
# Compute ACL names do not allow hyphens.
NAME="tf_test_migacl_$$_$(date +%s)"
PHASE=build
ACL_ID=""

api() {
    local method=$1 path=$2
    shift 2
    curl --fail --silent --show-error -X "$method" -H "Fastly-Key: $FASTLY_API_TOKEN" "$@" "$API_BASE$path"
}
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
    unset TF_LOG TF_LOG_PATH FASTLY_DEBUG_MODE
    if (( status != 0 )); then
        echo "[ERROR] Failed in phase $PHASE ($status)" >&2
        if [[ -f "$PROJECT/terraform.tfstate" ]]; then
            if grep -q 'resource "fastly_compute_acl"' "$PROJECT/main.tf"; then use_legacy; else use_target; fi
            (cd "$PROJECT" && terraform destroy -auto-approve -input=false) ||
                echo "Terraform cleanup failed; attempting API cleanup" >&2
        fi
        if [[ -n "$ACL_ID" ]]; then
            api DELETE "/resources/acls/$ACL_ID" >/dev/null 2>&1 || true
        fi
    fi
    if [[ -n "${KEEP_WORK_DIR:-}" || $status -ne 0 ]]; then
        echo "Private work directory retained: $WORK_DIR"
    else
        rm -rf -- "$WORK_DIR"
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
unset TF_LOG TF_LOG_PATH FASTLY_DEBUG_MODE TF_DATA_DIR TF_WORKSPACE TF_CLI_ARGS TF_CLI_ARGS_init TF_CLI_ARGS_plan TF_CLI_ARGS_apply TF_CLI_ARGS_destroy
mkdir -p "$PROJECT" "$WORK_DIR/provider"
(cd "$REPO_ROOT" && go build -o "$WORK_DIR/provider/terraform-provider-fastly" .)
cat > "$WORK_DIR/legacy.tfrc" <<'RC'
provider_installation {
  direct {}
}
RC
cat > "$WORK_DIR/target.tfrc" <<RC
provider_installation {
  dev_overrides {
    "fastly/fastly" = "$WORK_DIR/provider"
  }
  direct {}
}
RC

PHASE=legacy
cat > "$PROJECT/main.tf" <<HCL
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "= $VERSION"
    }
  }
}
resource "fastly_compute_acl" "blocklist" {
  name = "$NAME"
}
output "acl" {
  value = {
    id   = fastly_compute_acl.blocklist.id
    name = fastly_compute_acl.blocklist.name
  }
}
HCL
use_legacy
cd "$PROJECT"
terraform init -input=false
terraform version -json | jq -e --arg v "$VERSION" '.provider_selections["registry.terraform.io/fastly/fastly"] == $v' >/dev/null
terraform apply -auto-approve -input=false
terraform output -json acl | jq -S . > "$WORK_DIR/acl-before.json"
ACL_ID="$(jq -r .id "$WORK_DIR/acl-before.json")"
[[ -n "$ACL_ID" ]]
terraform state pull | jq -e '[.resources[] | select(.mode=="managed") | .instances[] | select(.schema_version != 0)] | length == 0' >/dev/null
api PATCH "/resources/acls/$ACL_ID/entries" -H 'Content-Type: application/json' \
    --data '{"entries":[{"op":"create","prefix":"203.0.113.0/24","action":"BLOCK"}]}' >/dev/null

PHASE=rename
sed -i 's/resource "fastly_compute_acl" "blocklist"/resource "fastly_acl" "blocklist"/; s/fastly_compute_acl\.blocklist\./fastly_acl.blocklist./g; s/version = "= '"$VERSION"'"/version = ">= 20.0.0"/' main.tf
cat > moved.tf <<'HCL'
moved {
  from = fastly_compute_acl.blocklist
  to   = fastly_acl.blocklist
}
HCL
if grep -q 'fastly_compute_acl' main.tf; then
    echo "Legacy resource type remains after rename" >&2
    exit 1
fi

PHASE=target-plan
use_target
# dev_overrides stand in for the unreleased target provider; drop the legacy lock file.
rm -f .terraform.lock.hcl
terraform validate
TRACE="$WORK_DIR/provider-trace.log"
: > "$TRACE"
export FASTLY_DEBUG_MODE=true TF_LOG=TRACE TF_LOG_PATH="$TRACE"
terraform plan -input=false -out="$WORK_DIR/move.tfplan"
unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH
terraform show -json "$WORK_DIR/move.tfplan" > "$WORK_DIR/move-plan.json"
jq -e '[.resource_changes[] | {from:.previous_address,to:.address,actions:.change.actions}] ==
    [{from:"fastly_compute_acl.blocklist",to:"fastly_acl.blocklist",actions:["no-op"]}]' \
    "$WORK_DIR/move-plan.json" >/dev/null ||
    { echo "Expected exactly one no-op move. Plan: $(jq -c '[.resource_changes[] | {previous_address,address,actions:.change.actions}]' "$WORK_DIR/move-plan.json")" >&2; exit 1; }
calls="$(grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$TRACE" |
    sed -E 's/^.*"([A-Z]+ )/\1/' | sort -u || true)"
unsafe="$(awk '$1 !~ /^(GET|HEAD)$/ {print}' <<< "$calls")"
[[ -z "$unsafe" ]] || { echo "Unexpected mutating API requests while planning: $unsafe" >&2; exit 1; }

PHASE=target-apply
terraform apply -input=false "$WORK_DIR/move.tfplan"
terraform plan -input=false -detailed-exitcode
terraform output -json acl | jq -S . > "$WORK_DIR/acl-after.json"
cmp "$WORK_DIR/acl-before.json" "$WORK_DIR/acl-after.json"
terraform state pull | jq -e '[.resources[] | select(.mode=="managed") | .type] == ["fastly_acl"]' >/dev/null

PHASE=verify
body="$(api GET "/resources/acls/$ACL_ID/entries")"
jq -e '[.entries[] | select(.prefix == "203.0.113.0/24" and .action == "BLOCK")] | length == 1' >/dev/null <<< "$body" ||
    { echo "Entry added outside Terraform is missing. Response: $body" >&2; exit 1; }
echo "[INFO] One no-op move, unchanged ID and name, no mutating requests while planning, entries untouched"

PHASE=cleanup
terraform destroy -auto-approve -input=false
echo "[INFO] fastly_compute_acl migration from SDKv2 $VERSION PASSED"
