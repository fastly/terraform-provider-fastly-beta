#!/usr/bin/env bash
# Opt-in live standalone domain migration test. Creates and deletes two real
# services and six versionless domains. Requires FASTLY_API_TOKEN for an
# isolated test account that may create domains; this has not been verified
# against accounts without that entitlement.
#
# The services live in a separate legacy-only project, so they are never
# migrated: the test proves that migrating domains neither mutates them nor
# changes their active version.
set -euo pipefail
umask 077

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LEGACY_PROVIDER_VERSION:-10.3.0}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Use an exact legacy release" >&2; exit 1; }
: "${FASTLY_API_TOKEN:?Set FASTLY_API_TOKEN for an isolated test account}"
for cmd in go terraform jq cksum; do
    command -v "$cmd" >/dev/null || { echo "Missing command: $cmd" >&2; exit 1; }
done
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-domains.XXXXXX")"
SERVICES="$WORK_DIR/services"
DOMAINS="$WORK_DIR/domains"
PREFIX="tf-test-migdomains-$$-$(date +%s)"
SUFFIX="${PREFIX}.fastly-example.com"
PHASE=build
STARTED=false
use_legacy() {
    export TF_CLI_CONFIG_FILE="$WORK_DIR/legacy.tfrc" FASTLY_API_KEY="$FASTLY_API_TOKEN"
}
use_target() {
    export TF_CLI_CONFIG_FILE="$WORK_DIR/target.tfrc"
    unset FASTLY_API_KEY
}
destroy_all() {
    if [[ -f "$DOMAINS/terraform.tfstate" ]]; then
        if grep -Eq 'resource "fastly_domain_v1' "$DOMAINS/main.tf"; then
            use_legacy
        else
            use_target
        fi
        (cd "$DOMAINS" && terraform destroy -auto-approve -input=false) ||
            echo "Domain cleanup failed; check for domains ending in $SUFFIX" >&2
    fi
    if [[ -f "$SERVICES/terraform.tfstate" ]]; then
        use_legacy
        (cd "$SERVICES" && terraform destroy -auto-approve -input=false) ||
            echo "Service cleanup failed; attempting prefix sweep" >&2
    fi
}
cleanup() {
    local status=$?
    trap - EXIT INT TERM
    unset TF_LOG TF_LOG_PATH FASTLY_DEBUG_MODE
    if (( status != 0 )); then
        echo "[ERROR] Failed in phase $PHASE ($status)" >&2
        if [[ "$STARTED" == true ]]; then
            destroy_all
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
mkdir -p "$SERVICES" "$DOMAINS" "$WORK_DIR/provider" "$WORK_DIR/backups"
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
REQUIRED="terraform {
  required_providers {
    fastly = {
      source = \"fastly/fastly\"
      version = \"= $VERSION\"
    }
  }
}"
cat > "$SERVICES/main.tf" <<HCL
$REQUIRED
variable "prefix" {
  type = string
}
resource "fastly_service_vcl" "direct" {
  name          = "\${var.prefix}-direct"
  force_destroy = true
  domain {
    name = "\${var.prefix}-direct-svc.fastly-example.com"
  }
  backend {
    name    = "origin"
    address = "www.example.com"
    port    = 443
  }
}
resource "fastly_service_vcl" "legacy" {
  name          = "\${var.prefix}-legacy"
  force_destroy = true
  domain {
    name = "\${var.prefix}-legacy-svc.fastly-example.com"
  }
  backend {
    name    = "origin"
    address = "www.example.com"
    port    = 443
  }
}
output "services" {
  value = {
    direct         = fastly_service_vcl.direct.id
    direct_version = fastly_service_vcl.direct.active_version
    legacy         = fastly_service_vcl.legacy.id
    legacy_version = fastly_service_vcl.legacy.active_version
  }
}
HCL

# Covers: unchanged fastly_domain (direct association, omitted description,
# populated description, standalone link; the linked domain sets no description
# because creating the link leaves the remote description empty), both deprecated renames, and a
# standalone link for each family. domain_id references exercise the rewrite.
cat > "$DOMAINS/main.tf" <<HCL
$REQUIRED
variable "suffix" {
  type = string
}
variable "service_direct" {
  type = string
}
variable "service_legacy" {
  type = string
}
resource "fastly_domain" "direct" {
  fqdn        = "direct.\${var.suffix}"
  description = "direct association"
  service_id  = var.service_direct
}
resource "fastly_domain" "omitted" {
  fqdn = "omitted.\${var.suffix}"
}
resource "fastly_domain" "linked" {
  fqdn = "linked.\${var.suffix}"
}
resource "fastly_domain_service_link" "linked" {
  domain_id  = fastly_domain.linked.domain_id
  service_id = var.service_direct
}
resource "fastly_domain_v1" "v1_associated" {
  fqdn        = "v1.\${var.suffix}"
  description = "renamed with association"
  service_id  = var.service_legacy
}
resource "fastly_domain_v1" "v1_omitted" {
  fqdn = "v1-omitted.\${var.suffix}"
}
resource "fastly_domain_v1" "v1_linked" {
  fqdn = "v1-linked.\${var.suffix}"
}
resource "fastly_domain_v1_service_link" "v1_link" {
  domain_id  = fastly_domain_v1.v1_linked.domain_id
  service_id = var.service_legacy
}
output "domains" {
  value = {
    direct        = fastly_domain.direct.domain_id
    omitted       = fastly_domain.omitted.domain_id
    linked        = fastly_domain.linked.domain_id
    link          = fastly_domain_service_link.linked.id
    v1_associated = fastly_domain_v1.v1_associated.domain_id
    v1_omitted    = fastly_domain_v1.v1_omitted.domain_id
    v1_linked     = fastly_domain_v1.v1_linked.domain_id
    v1_link       = fastly_domain_v1_service_link.v1_link.id
  }
}
HCL

export TF_VAR_prefix="$PREFIX" TF_VAR_suffix="$SUFFIX"
PHASE=legacy-services
use_legacy
cd "$SERVICES"
terraform init -input=false
STARTED=true
terraform apply -auto-approve -input=false
terraform output -json services | jq -S . > "$WORK_DIR/services-before.json"
jq -e '.direct != "" and .legacy != "" and .direct_version >= 1 and .legacy_version >= 1' "$WORK_DIR/services-before.json" >/dev/null
TF_VAR_service_direct="$(jq -r .direct "$WORK_DIR/services-before.json")"
TF_VAR_service_legacy="$(jq -r .legacy "$WORK_DIR/services-before.json")"
export TF_VAR_service_direct TF_VAR_service_legacy

PHASE=legacy-domains
cd "$DOMAINS"
terraform init -input=false
terraform version -json | jq -e --arg v "$VERSION" '.provider_selections["registry.terraform.io/fastly/fastly"] == $v' >/dev/null
terraform apply -auto-approve -input=false
terraform apply -refresh-only -auto-approve -input=false
terraform output -json domains | jq -S . > "$WORK_DIR/domains-before.json"
jq -e 'all(.[]; . != "")' "$WORK_DIR/domains-before.json" >/dev/null
terraform state pull > "$WORK_DIR/state-before.json"
# Every attribute a migration must preserve, keyed by the target address. The
# legacy-only domain_id is dropped from domains, and SDKv2's empty description
# and service_id are the same unset values as the target's null.
NORMALIZE='[.resources[] | select(.mode=="managed") | . as $r | .instances[] as $i |
    {key: (($r.type | sub("_v1"; "")) + "." + $r.name),
     value: (if ($r.type | test("service_link")) then $i.attributes
             else ($i.attributes | del(.domain_id) | .description |= (if . == "" then null else . end)
                                  | .service_id |= (if . == "" then null else . end)) end)}]
    | from_entries'
jq -S "$NORMALIZE" "$WORK_DIR/state-before.json" > "$WORK_DIR/attributes-before.json"
jq -e '[.resources[] | select(.mode=="managed") | .instances[] | select(.schema_version != 0)] | length == 0' \
    "$WORK_DIR/state-before.json" >/dev/null
jq -e '.resources[] | select(.type=="fastly_domain" and .name=="omitted") | .instances[0].attributes.description == ""' \
    "$WORK_DIR/state-before.json" >/dev/null ||
    echo "[WARN] SDKv2 stored a null description for an omitted value; the empty-string case is not exercised by this run" >&2

PHASE=cli
"$WORK_DIR/migrator" analyze .
before="$(cksum main.tf .terraform.lock.hcl)"
"$WORK_DIR/migrator" migrate --dry-run . | tee "$WORK_DIR/preview.txt"
grep -Fxq 'Migration dry-run: READY' "$WORK_DIR/preview.txt"
grep -Fxq 'Resources to move: 4' "$WORK_DIR/preview.txt"
[[ "$before" == "$(cksum main.tf .terraform.lock.hcl)" ]]
[[ ! -e migration.tf ]]
"$WORK_DIR/migrator" migrate --write --backup-dir "$WORK_DIR/backups" .
if grep -Eq 'fastly_domain_v1|\.domain_id' main.tf; then
    echo "Obsolete domain resource or reference remains" >&2
    exit 1
fi
[[ -n "$(find "$WORK_DIR/backups" -type f -name manifest.json -print -quit)" ]]
# A second analysis must not recommend another migration or touch the files.
migrated="$(cksum main.tf migration.tf)"
"$WORK_DIR/migrator" migrate --dry-run . | tee "$WORK_DIR/preview-again.txt"
grep -Fxq 'Migration dry-run: READY' "$WORK_DIR/preview-again.txt"
grep -Fxq 'Resources to move: 0' "$WORK_DIR/preview-again.txt"
[[ "$migrated" == "$(cksum main.tf migration.tf)" ]]

PHASE=target
use_target
# dev_overrides stand in for the unreleased target provider. Skip init and drop
# the legacy lock file, as the reference migration test does.
rm -f .terraform.lock.hcl
terraform validate
TRACE="$WORK_DIR/migration-provider-trace.log"
: > "$TRACE"
export FASTLY_DEBUG_MODE=true TF_LOG=TRACE TF_LOG_PATH="$TRACE"
terraform plan -input=false -out="$WORK_DIR/move.tfplan"
# Zero infrastructure-changing actions: only no-op and read.
terraform show -json "$WORK_DIR/move.tfplan" | jq -e '
    [.resource_changes[]? | select(
        (.change.actions | all(. == "no-op" or . == "read") | not) or
        (.change.importing != null) or
        (.mode == "managed" and .change.actions != ["no-op"]))] | length == 0' >/dev/null
terraform show -json "$WORK_DIR/move.tfplan" | jq -e '
    [.resource_changes[]? | select(.previous_address != null) | {from:.previous_address,to:.address}] |
    sort_by(.from) == [
      {from:"fastly_domain_v1.v1_associated",to:"fastly_domain.v1_associated"},
      {from:"fastly_domain_v1.v1_linked",to:"fastly_domain.v1_linked"},
      {from:"fastly_domain_v1.v1_omitted",to:"fastly_domain.v1_omitted"},
      {from:"fastly_domain_v1_service_link.v1_link",to:"fastly_domain_service_link.v1_link"}
    ]' >/dev/null
terraform apply -input=false "$WORK_DIR/move.tfplan"
terraform plan -input=false -detailed-exitcode
terraform output -json domains | jq -S . > "$WORK_DIR/domains-after.json"
cmp "$WORK_DIR/domains-before.json" "$WORK_DIR/domains-after.json"
unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH

PHASE=verify
terraform state pull > "$WORK_DIR/state-after.json"
jq -e '[.resources[] | select(.mode=="managed") | .type] | all(. == "fastly_domain" or . == "fastly_domain_service_link")' \
    "$WORK_DIR/state-after.json" >/dev/null
# Exact state preservation: same ids, fqdns, descriptions and service ids.
jq -S "$NORMALIZE" "$WORK_DIR/state-after.json" > "$WORK_DIR/attributes-after.json"
cmp "$WORK_DIR/attributes-before.json" "$WORK_DIR/attributes-after.json"
# The legacy-only domain_id attribute is gone from every migrated domain.
jq -e '[.resources[] | select(.type=="fastly_domain") | .instances[].attributes | has("domain_id")] | any | not' \
    "$WORK_DIR/state-after.json" >/dev/null
# Unlinked domains stay unlinked; linked ones keep the original service id.
jq -e '
    [.resources[] | select(.type=="fastly_domain" and .name=="omitted") | .instances[0].attributes.service_id] == [null]' \
    "$WORK_DIR/state-after.json" >/dev/null
jq -e --arg svc "$TF_VAR_service_direct" '
    [.resources[] | select(.type=="fastly_domain" and .name=="direct") | .instances[0].attributes.service_id] == [$svc]' \
    "$WORK_DIR/state-after.json" >/dev/null
calls="$(grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$TRACE" |
    sed -E 's/^.*"([A-Z]+ )/\1/' | sort -u || true)"
awk '$1 ~ /^(GET|HEAD)$/ && $2 ~ /^\/domain-management\/v1\// {found=1} END {exit !found}' <<< "$calls"
unsafe="$(awk '$1 !~ /^(GET|HEAD)$/ && $2 ~ /^\/(domain-management\/v1|service|enabled-products)(\/|$)/ {print}' <<< "$calls")"
[[ -z "$unsafe" ]] || { echo "Unexpected mutating domain/service API requests: $unsafe" >&2; exit 1; }
terraform state list > "$WORK_DIR/addresses.txt"
if grep -Eq '^fastly_domain_v1' "$WORK_DIR/addresses.txt"; then
    echo "Legacy state address remains" >&2
    exit 1
fi
# The services were never migrated: ids and active versions must be unchanged.
use_legacy
(cd "$SERVICES" && terraform apply -refresh-only -auto-approve -input=false &&
    terraform output -json services | jq -S . > "$WORK_DIR/services-after.json")
cmp "$WORK_DIR/services-before.json" "$WORK_DIR/services-after.json"
echo "[INFO] Exact state moves, stable outputs, no mutating domain/service requests"

PHASE=cleanup
destroy_all
echo "[INFO] Standalone domain migration from SDKv2 $VERSION PASSED"
