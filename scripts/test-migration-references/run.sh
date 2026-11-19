#!/usr/bin/env bash
# Opt-in live reference migration test. Creates and deletes two real services.
set -euo pipefail
umask 077

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LEGACY_PROVIDER_VERSION:-10.3.0}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Use an exact legacy release" >&2; exit 1; }
: "${FASTLY_API_TOKEN:?Set FASTLY_API_TOKEN for an isolated test account}"
for cmd in go terraform jq cksum; do
    command -v "$cmd" >/dev/null || { echo "Missing command: $cmd" >&2; exit 1; }
done
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-refs.XXXXXX")"
PROJECT="$WORK_DIR/project"
PREFIX="tf-test-migrefs-$$-$(date +%s)"
PHASE=build
STARTED=false
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
        if [[ "$STARTED" == true ]]; then
            if [[ -f "$PROJECT/terraform.tfstate" ]]; then
                if grep -Fq 'resource "fastly_service_cdn_auto"' "$PROJECT/services/main.tf"; then
                    use_target
                else
                    use_legacy
                fi
                (cd "$PROJECT" && terraform destroy -auto-approve -input=false) ||
                    echo "Terraform cleanup failed; attempting prefix sweep" >&2
            fi
            (cd "$REPO_ROOT" && go run ./scripts/cleanup-test-services -prefix "$PREFIX") ||
                echo "Check for remaining services named $PREFIX*" >&2
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
mkdir -p "$PROJECT/services" "$WORK_DIR/provider" "$WORK_DIR/backups"
(cd "$REPO_ROOT" &&
    go build -o "$WORK_DIR/provider/terraform-provider-fastly" . &&
    go build -o "$WORK_DIR/migrator" ./cmd/fastly-tf-migrate)
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
PHASE=config
cd "$PROJECT"
for file in main.tf services/main.tf; do
    cat > "$file" <<HCL
terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "= $VERSION"
    }
  }
}
HCL
done
cat >> main.tf <<'HCL'
provider "fastly" {
  alias = "edge"
}
variable "test_prefix" {
  type = string
}
module "edge" {
  source = "./services"
  for_each = toset(["production"])
  providers = { fastly = fastly.edge }
  prefix = "${var.test_prefix}-${each.key}"
}
output "references" {
  value = module.edge["production"].identity
}
HCL
cat >> services/main.tf <<'HCL'
variable "prefix" {
  type = string
}
resource "fastly_service_vcl" "counted" {
  count = 1
  name = "${var.prefix}-vcl-${count.index}"
  force_destroy = true
  domain {
    name = "${var.prefix}-vcl-${count.index}.example.com"
  }
  backend {
    name = "origin"
    address = "www.example.com"
    port = 443
  }
  lifecycle {
    ignore_changes = [comment]
  }
}
data "fastly_package_hash" "pkg" {
  filename = "${path.module}/package.tar.gz"
}
resource "fastly_service_compute" "keyed" {
  for_each = toset(["blue"])
  name = "${var.prefix}-compute-${each.key}"
  force_destroy = true
  domain {
    name = "${var.prefix}-compute-${each.key}.example.com"
  }
  backend {
    name = "origin"
    address = "www.example.com"
    port = 443
  }
  package {
    filename = "${path.module}/package.tar.gz"
    source_code_hash = data.fastly_package_hash.pkg.hash
  }
}
HCL
cat > services/references.tf <<'HCL'
locals {
  vcl_ids = fastly_service_vcl.counted[*].id
  compute_ids = {for k, svc in fastly_service_compute.keyed : k => svc.id}
}
resource "terraform_data" "consumer" {
  for_each = toset(["blue"])
  input = "${fastly_service_vcl.counted[0].id}:${fastly_service_compute.keyed[each.key].id}"
  depends_on = [fastly_service_vcl.counted, fastly_service_compute.keyed]
}
output "identity" {
  value = {
    vcl_ids = local.vcl_ids
    compute_ids = local.compute_ids
    vcl_version = fastly_service_vcl.counted[0].active_version
    compute_version = fastly_service_compute.keyed["blue"].active_version
    consumer_id = terraform_data.consumer["blue"].id
    consumer_input = terraform_data.consumer["blue"].input
  }
}
HCL
cp "$REPO_ROOT/internal/acceptance_tests/fixtures/packages/valid.tar.gz" services/package.tar.gz
export TF_VAR_test_prefix="$PREFIX"
PHASE=legacy
use_legacy
terraform init -input=false
terraform version -json | jq -e --arg v "$VERSION" '.provider_selections["registry.terraform.io/fastly/fastly"] == $v' >/dev/null
STARTED=true
terraform apply -auto-approve -input=false
terraform apply -refresh-only -auto-approve -input=false
terraform output -json references | jq -S . > "$WORK_DIR/before.json"
jq -e '.vcl_ids[0] != "" and .compute_ids.blue != "" and .vcl_version >= 1 and .compute_version >= 1' "$WORK_DIR/before.json" >/dev/null
terraform state pull | jq -e '
  [.resources[] | select(.type=="fastly_service_vcl") | .instances[] | select(.schema_version==1)] | length==1' >/dev/null
terraform state pull | jq -e '
  [.resources[] | select(.type=="fastly_service_compute") | .instances[] | select(.schema_version==0)] | length==1' >/dev/null
PHASE=cli
"$WORK_DIR/migrator" analyze .
before="$(cksum main.tf services/main.tf services/references.tf .terraform.lock.hcl)"
"$WORK_DIR/migrator" migrate --dry-run . | tee "$WORK_DIR/preview.txt"
grep -Fxq 'Migration dry-run: READY' "$WORK_DIR/preview.txt"
grep -Fxq 'Resources to move: 2' "$WORK_DIR/preview.txt"
[[ "$before" == "$(cksum main.tf services/main.tf services/references.tf .terraform.lock.hcl)" ]]
[[ ! -e migration.tf && ! -e services/migration.tf ]]
lock="$(cksum .terraform.lock.hcl)"
"$WORK_DIR/migrator" migrate --write --backup-dir "$WORK_DIR/backups" .
[[ "$lock" == "$(cksum .terraform.lock.hcl)" ]]
grep -Fq 'fastly_service_cdn_auto.counted' services/references.tf
grep -Fq 'fastly_service_compute_auto.keyed' services/references.tf
if grep -Eq 'fastly_service_(vcl|compute)\.' services/references.tf; then
    echo "Obsolete service reference remains" >&2
    exit 1
fi
[[ -n "$(find "$WORK_DIR/backups" -type f -name manifest.json -print -quit)" ]]
PHASE=target
use_target
# The local dev override represents the not-yet-published target release.
rm -f .terraform.lock.hcl
terraform validate
TRACE="$WORK_DIR/migration-provider-trace.log"
: > "$TRACE"
export FASTLY_DEBUG_MODE=true TF_LOG=TRACE TF_LOG_PATH="$TRACE"
terraform plan -input=false -out="$WORK_DIR/move.tfplan"
terraform show -json "$WORK_DIR/move.tfplan" | jq -e '
    [.resource_changes[]? | select(
        (.change.actions | all(. == "no-op" or . == "read") | not) or
        (.change.importing != null) or
        (.mode == "managed" and .change.actions != ["no-op"]))] | length==0' >/dev/null
terraform show -json "$WORK_DIR/move.tfplan" | jq -e '
    [.resource_changes[]? | select(.previous_address != null) | {from:.previous_address,to:.address}] |
    sort_by(.from) == [
      {from:"module.edge[\"production\"].fastly_service_compute.keyed[\"blue\"]",to:"module.edge[\"production\"].fastly_service_compute_auto.keyed[\"blue\"]"},
      {from:"module.edge[\"production\"].fastly_service_vcl.counted[0]",to:"module.edge[\"production\"].fastly_service_cdn_auto.counted[0]"}
    ]' >/dev/null
terraform apply -input=false "$WORK_DIR/move.tfplan"
terraform plan -input=false -detailed-exitcode
terraform output -json references | jq -S . > "$WORK_DIR/after.json"
cmp "$WORK_DIR/before.json" "$WORK_DIR/after.json"
unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH
calls="$(grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$TRACE" |
    sed -E 's/^.*"([A-Z]+ )/\1/' | sort -u || true)"
for id in "$(jq -r '.vcl_ids[0]' "$WORK_DIR/before.json")" \
          "$(jq -r '.compute_ids.blue' "$WORK_DIR/before.json")"; do
    awk -v id="$id" '$1 ~ /^(GET|HEAD)$/ && index($2, "/service/" id) == 1 {found=1} END {exit !found}' <<< "$calls"
done
unsafe="$(awk '$1 !~ /^(GET|HEAD)$/ && $2 ~ /^\/(service|enabled-products)(\/|$)/ {print}' <<< "$calls")"
[[ -z "$unsafe" ]] || { echo "Unexpected mutating service/product API requests: $unsafe" >&2; exit 1; }
terraform state list > "$WORK_DIR/addresses.txt"
grep -Fxq 'module.edge["production"].fastly_service_cdn_auto.counted[0]' "$WORK_DIR/addresses.txt"
grep -Fxq 'module.edge["production"].fastly_service_compute_auto.keyed["blue"]' "$WORK_DIR/addresses.txt"
if grep -Eq '\.fastly_service_(vcl|compute)\.' "$WORK_DIR/addresses.txt"; then
    echo "Legacy state address remains" >&2
    exit 1
fi
echo "[INFO] Exact instance moves, stable outputs, no mutating service/product requests"
PHASE=cleanup
terraform destroy -auto-approve -input=false
echo "[INFO] Reference migration from SDKv2 $VERSION PASSED"
