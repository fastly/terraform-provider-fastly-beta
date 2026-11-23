#!/usr/bin/env bash
# Opt-in live migration test for the renamed ACL and Config Store containers.
# Creates and deletes one VCL service, two Compute ACLs and two Config Stores.
# Requires FASTLY_API_TOKEN for an isolated test account.
#
# Each legacy entry resource exists twice: with manage_entries = true and
# seeded-only (manage_entries omitted). An undeclared entry is added outside
# Terraform to every seeded container; migration must never remove it.
set -euo pipefail
umask 077

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LEGACY_PROVIDER_VERSION:-10.3.0}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Use an exact legacy release" >&2; exit 1; }
: "${FASTLY_API_TOKEN:?Set FASTLY_API_TOKEN for an isolated test account}"
for cmd in go terraform jq curl cksum; do
    command -v "$cmd" >/dev/null || { echo "Missing command: $cmd" >&2; exit 1; }
done
API_BASE="${FASTLY_API_URL:-https://api.fastly.com}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-containers.XXXXXX")"
SERVICES="$WORK_DIR/services"
CONTAINERS="$WORK_DIR/containers"
PREFIX="tf-test-migcontainers-$$-$(date +%s)"
# Compute ACL and Config Store names do not allow hyphens.
NAME_PREFIX="${PREFIX//-/_}"
PHASE=build
STARTED=false

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
destroy_all() {
    if [[ -f "$CONTAINERS/terraform.tfstate" ]]; then
        if grep -Eq 'resource "fastly_(compute_acl|configstore_entries|service_acl_entries)"' "$CONTAINERS/main.tf"; then
            use_legacy
        else
            use_target
        fi
        (cd "$CONTAINERS" && terraform destroy -auto-approve -input=false) ||
            echo "Container cleanup failed; attempting API cleanup" >&2
    fi
    # Undeclared entries can block a legacy Config Store delete.
    if [[ -f "$WORK_DIR/containers-before.json" ]]; then
        for id in $(jq -r '.managed_acl, .seeded_acl' "$WORK_DIR/containers-before.json"); do
            api DELETE "/resources/acls/$id" >/dev/null 2>&1 || true
        done
        for id in $(jq -r '.managed_store, .seeded_store' "$WORK_DIR/containers-before.json"); do
            api DELETE "/resources/stores/config/$id" >/dev/null 2>&1 || true
        done
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
                echo "Check for remaining services named $PREFIX* and ACLs or Config Stores named $NAME_PREFIX*" >&2
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
mkdir -p "$SERVICES" "$CONTAINERS" "$WORK_DIR/provider" "$WORK_DIR/backups"
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
# The service lives in a separate legacy-only project and is never migrated.
cat > "$SERVICES/main.tf" <<HCL
$REQUIRED
variable "prefix" {
  type = string
}
resource "fastly_service_vcl" "acls" {
  name          = "\${var.prefix}-acls"
  force_destroy = true
  domain {
    name = "\${var.prefix}-acls.fastly-example.com"
  }
  backend {
    name    = "origin"
    address = "www.example.com"
    port    = 443
  }
  acl {
    name          = "managed"
    force_destroy = true
  }
  acl {
    name          = "seeded"
    force_destroy = true
  }
}
output "service" {
  value = {
    id      = fastly_service_vcl.acls.id
    version = fastly_service_vcl.acls.active_version
    acls    = { for acl in fastly_service_vcl.acls.acl : acl.name => acl.acl_id }
  }
}
HCL

cat > "$CONTAINERS/main.tf" <<HCL
$REQUIRED
variable "name_prefix" {
  type = string
}
variable "service_id" {
  type = string
}
variable "managed_acl_id" {
  type = string
}
variable "seeded_acl_id" {
  type = string
}
resource "fastly_compute_acl" "managed" {
  name = "\${var.name_prefix}_managed"
}
resource "fastly_compute_acl_entries" "managed" {
  compute_acl_id = fastly_compute_acl.managed.id
  entries = {
    "192.0.2.0/24" = "BLOCK"
  }
  manage_entries = true
}
resource "fastly_compute_acl" "seeded" {
  name = "\${var.name_prefix}_seeded"
}
resource "fastly_compute_acl_entries" "seeded" {
  compute_acl_id = fastly_compute_acl.seeded.id
  entries = {
    "198.51.100.0/24" = "ALLOW"
  }
}
resource "fastly_configstore" "managed" {
  name = "\${var.name_prefix}_managed"
}
resource "fastly_configstore_entries" "managed" {
  store_id = fastly_configstore.managed.id
  entries = {
    mode = "strict"
  }
  manage_entries = true
}
resource "fastly_configstore" "seeded" {
  name = "\${var.name_prefix}_seeded"
}
resource "fastly_configstore_entries" "seeded" {
  store_id = fastly_configstore.seeded.id
  entries = {
    mode = "relaxed"
  }
}
resource "fastly_service_acl_entries" "managed" {
  service_id     = var.service_id
  acl_id         = var.managed_acl_id
  manage_entries = true
  entry {
    ip      = "192.0.2.10"
    subnet  = "32"
    comment = "managed"
  }
}
resource "fastly_service_acl_entries" "seeded" {
  service_id = var.service_id
  acl_id     = var.seeded_acl_id
  entry {
    ip      = "198.51.100.10"
    comment = "seeded"
  }
}
output "containers" {
  value = {
    managed_acl        = fastly_compute_acl.managed.id
    managed_acl_name   = fastly_compute_acl.managed.name
    seeded_acl         = fastly_compute_acl_entries.seeded.compute_acl_id
    managed_store      = fastly_configstore.managed.id
    seeded_store       = fastly_configstore_entries.seeded.store_id
    managed_service    = fastly_service_acl_entries.managed.service_id
    seeded_service_acl = fastly_service_acl_entries.seeded.acl_id
  }
}
HCL

export TF_VAR_prefix="$PREFIX" TF_VAR_name_prefix="$NAME_PREFIX"
PHASE=legacy-service
use_legacy
cd "$SERVICES"
terraform init -input=false
STARTED=true
terraform apply -auto-approve -input=false
terraform output -json service | jq -S . > "$WORK_DIR/service-before.json"
jq -e '.id != "" and .version >= 1 and .acls.managed != "" and .acls.seeded != ""' "$WORK_DIR/service-before.json" >/dev/null
SERVICE_ID="$(jq -r .id "$WORK_DIR/service-before.json")"
SEEDED_SERVICE_ACL="$(jq -r .acls.seeded "$WORK_DIR/service-before.json")"
export TF_VAR_service_id="$SERVICE_ID" TF_VAR_seeded_acl_id="$SEEDED_SERVICE_ACL"
TF_VAR_managed_acl_id="$(jq -r .acls.managed "$WORK_DIR/service-before.json")"
export TF_VAR_managed_acl_id

PHASE=legacy-containers
cd "$CONTAINERS"
terraform init -input=false
terraform version -json | jq -e --arg v "$VERSION" '.provider_selections["registry.terraform.io/fastly/fastly"] == $v' >/dev/null
terraform apply -auto-approve -input=false
terraform output -json containers | jq -S . > "$WORK_DIR/containers-before.json"
jq -e 'all(.[]; . != "")' "$WORK_DIR/containers-before.json" >/dev/null
SEEDED_ACL="$(jq -r .seeded_acl "$WORK_DIR/containers-before.json")"
SEEDED_STORE="$(jq -r .seeded_store "$WORK_DIR/containers-before.json")"

PHASE=out-of-band
api PATCH "/resources/acls/$SEEDED_ACL/entries" -H 'Content-Type: application/json' \
    --data '{"entries":[{"op":"create","prefix":"203.0.113.0/24","action":"BLOCK"}]}' >/dev/null
api POST "/resources/stores/config/$SEEDED_STORE/item" \
    --data-urlencode item_key=external --data-urlencode item_value=outside-terraform >/dev/null
api POST "/service/$SERVICE_ID/acl/$SEEDED_SERVICE_ACL/entry" \
    --data-urlencode ip=203.0.113.20 --data-urlencode comment=outside-terraform >/dev/null
# Seeded-only legacy resources ignore entries added outside Terraform.
terraform apply -refresh-only -auto-approve -input=false
terraform plan -input=false -detailed-exitcode
terraform state pull > "$WORK_DIR/state-before.json"
jq -e '[.resources[] | select(.mode=="managed") | .instances[] | select(.schema_version != 0)] | length == 0' \
    "$WORK_DIR/state-before.json" >/dev/null

PHASE=cli
"$WORK_DIR/migrator" analyze .
before="$(cksum main.tf .terraform.lock.hcl)"
"$WORK_DIR/migrator" migrate --dry-run . | tee "$WORK_DIR/preview.txt"
grep -Fxq 'Migration dry-run: READY' "$WORK_DIR/preview.txt"
grep -Fxq 'Resources to move: 8' "$WORK_DIR/preview.txt"
# One warning for each seeded-only entry resource.
[[ "$(grep -c '^  WARNING .*in sync with your configuration' "$WORK_DIR/preview.txt")" == 3 ]]
[[ "$before" == "$(cksum main.tf .terraform.lock.hcl)" ]]
[[ ! -e migration.tf ]]
"$WORK_DIR/migrator" migrate --write --backup-dir "$WORK_DIR/backups" .
if grep -Eq 'fastly_compute_acl|fastly_configstore_entries|fastly_service_acl_entries|manage_entries|compute_acl_id' main.tf; then
    echo "Obsolete container resource, attribute or reference remains" >&2
    exit 1
fi
[[ -n "$(find "$WORK_DIR/backups" -type f -name manifest.json -print -quit)" ]]
migrated="$(cksum main.tf migration.tf)"
"$WORK_DIR/migrator" migrate --dry-run . | tee "$WORK_DIR/preview-again.txt"
grep -Fxq 'Migration dry-run: READY' "$WORK_DIR/preview-again.txt"
grep -Fxq 'Resources to move: 0' "$WORK_DIR/preview-again.txt"
[[ "$migrated" == "$(cksum main.tf migration.tf)" ]]

PHASE=target-plan
use_target
rm -f .terraform.lock.hcl
terraform validate
TRACE="$WORK_DIR/migration-provider-trace.log"
: > "$TRACE"
export FASTLY_DEBUG_MODE=true TF_LOG=TRACE TF_LOG_PATH="$TRACE"
terraform plan -input=false -out="$WORK_DIR/move.tfplan"
unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH
terraform show -json "$WORK_DIR/move.tfplan" > "$WORK_DIR/move-plan.json"
jq -e '
    [.resource_changes[]? | select(.previous_address != null) | {from:.previous_address,to:.address}] |
    sort_by(.from) == [
      {from:"fastly_compute_acl.managed",to:"fastly_acl.managed"},
      {from:"fastly_compute_acl.seeded",to:"fastly_acl.seeded"},
      {from:"fastly_compute_acl_entries.managed",to:"fastly_acl_entries.managed"},
      {from:"fastly_compute_acl_entries.seeded",to:"fastly_acl_entries.seeded"},
      {from:"fastly_configstore_entries.managed",to:"fastly_configstore_items.managed"},
      {from:"fastly_configstore_entries.seeded",to:"fastly_configstore_items.seeded"},
      {from:"fastly_service_acl_entries.managed",to:"fastly_service_cdn_acl_entries.managed"},
      {from:"fastly_service_acl_entries.seeded",to:"fastly_service_cdn_acl_entries.seeded"}
    ]' "$WORK_DIR/move-plan.json" >/dev/null
# Nothing is created, destroyed or imported. Seeded entry resources start
# owning nothing, so they alone update to re-declare their entries.
jq -e '
    [.resource_changes[]? | select(.mode == "managed") |
        select(.change.importing != null or
            (if (.name == "seeded" and (.type | test("_(entries|items)$")))
             then .change.actions != ["update"]
             else .change.actions != ["no-op"] end))] | length == 0' "$WORK_DIR/move-plan.json" >/dev/null
calls="$(grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$TRACE" |
    sed -E 's/^.*"([A-Z]+ )/\1/' | sort -u || true)"
unsafe="$(awk '$1 !~ /^(GET|HEAD)$/ {print}' <<< "$calls")"
[[ -z "$unsafe" ]] || { echo "Unexpected mutating API requests while planning: $unsafe" >&2; exit 1; }

PHASE=target-apply
terraform apply -input=false "$WORK_DIR/move.tfplan"
terraform plan -input=false -detailed-exitcode
terraform output -json containers | jq -S . > "$WORK_DIR/containers-after.json"
cmp "$WORK_DIR/containers-before.json" "$WORK_DIR/containers-after.json"

PHASE=verify
# require_body checks a response body and prints it when the check fails.
require_body() {
    local what=$1 filter=$2 body=$3
    jq -e "$filter" >/dev/null <<< "$body" || { echo "$what is missing. Response: $body" >&2; return 1; }
}
undeclared_survived() {
    require_body "Undeclared Compute ACL entry 203.0.113.0/24" \
        '[.entries[] | select(.prefix == "203.0.113.0/24")] | length == 1' \
        "$(api GET "/resources/acls/$SEEDED_ACL/entries")"
    require_body "Undeclared Config Store item external" \
        '.item_value == "outside-terraform"' \
        "$(api GET "/resources/stores/config/$SEEDED_STORE/item/external")"
    require_body "Undeclared service ACL entry 203.0.113.20" \
        '[.[] | select(.ip == "203.0.113.20")] | length == 1' \
        "$(api GET "/service/$SERVICE_ID/acl/$SEEDED_SERVICE_ACL/entries?per_page=100")"
}
undeclared_survived
terraform state pull > "$WORK_DIR/state-after.json"
jq -e '[.resources[] | select(.mode=="managed") | .type] | all(test("^fastly_(acl|acl_entries|configstore|configstore_items|service_cdn_acl_entries)$"))' \
    "$WORK_DIR/state-after.json" >/dev/null

# Remove the only owned entry outside Terraform, then save a refresh: the
# empty owned set must stay empty instead of adopting the undeclared entry.
PHASE=refresh-only
owned_entry="$(api GET "/service/$SERVICE_ID/acl/$SEEDED_SERVICE_ACL/entries" |
    jq -r '.[] | select(.ip == "198.51.100.10") | .id')"
[[ -n "$owned_entry" ]]
api DELETE "/service/$SERVICE_ID/acl/$SEEDED_SERVICE_ACL/entry/$owned_entry" >/dev/null
terraform apply -refresh-only -auto-approve -input=false
terraform state pull | jq -e '
    [.resources[] | select(.type == "fastly_service_cdn_acl_entries" and .name == "seeded") |
        .instances[0].attributes.entry] == [[]]' >/dev/null
terraform plan -input=false -out="$WORK_DIR/readd.tfplan"
terraform show -json "$WORK_DIR/readd.tfplan" | jq -e '
    [.resource_changes[]? | select(.mode == "managed" and .change.actions != ["no-op"]) | .address] ==
        ["fastly_service_cdn_acl_entries.seeded"]' >/dev/null
terraform apply -input=false "$WORK_DIR/readd.tfplan"
terraform plan -input=false -detailed-exitcode
undeclared_survived

# The service was never migrated: its id and active version are unchanged.
use_legacy
(cd "$SERVICES" && terraform apply -refresh-only -auto-approve -input=false &&
    terraform output -json service | jq -S . > "$WORK_DIR/service-after.json")
cmp "$WORK_DIR/service-before.json" "$WORK_DIR/service-after.json"
echo "[INFO] Exact moves, undeclared entries preserved, refresh-only keeps an empty owned set empty"

PHASE=cleanup
destroy_all
echo "[INFO] Container migration from SDKv2 $VERSION PASSED"
