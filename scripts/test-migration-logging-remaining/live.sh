#!/usr/bin/env bash
# Opt-in, API-backed SDKv2 -> CDN Auto VCL-07 logging migration.
# Scenarios: Papertrail (no receiver credentials), FOS (test-owned bucket and entitlement).
# Creates and destroys one disposable Fastly service per invocation.
set -euo pipefail
umask 077

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LEGACY_PROVIDER_VERSION:-10.3.0}"
API_BASE="${FASTLY_API_URL:-https://api.fastly.com}"
PHASE="preflight"
CREATION_STARTED=false
SCENARIO="${1:-papertrail}"
info() { printf '[INFO] %s\n' "$*"; }
fail() { printf '[ERROR] %s (phase: %s)\n' "$*" "$PHASE" >&2; exit 1; }
usage() {
    cat <<'HELP'
Opt-in VCL-07 migration test using a disposable SDKv2 CDN service.

Usage: live.sh papertrail|fos

Requires FASTLY_API_TOKEN authorized to create and destroy disposable services.
FOS additionally requires FASTLY_FOS_TEST_BUCKET and FASTLY_FOS_TEST_REGION
for a bucket owned by the test account and the relevant FOS entitlement.
No actual logging delivery to a receiver is tested.

The script verifies legacy and target no-op plans, state move identity,
endpoint configuration, unchanged active service version and no mutating
service/product API calls during migration.
HELP
}
[[ "${1:-}" == "--help" ]] && { usage; exit 0; }
[[ $# -le 1 ]] || { usage >&2; exit 2; }
case "$SCENARIO" in
    papertrail) LOGGING_BLOCK=logging_papertrail; ENDPOINT_NAME=papertrail-live ;;
    fos) LOGGING_BLOCK=logging_fos; ENDPOINT_NAME=fos-live ;;
    *) usage >&2; exit 2 ;;
esac
LEGACY_TYPE=fastly_service_vcl
TARGET_TYPE=fastly_service_cdn_auto
LEGACY_ADDRESS="$LEGACY_TYPE.demo"
TARGET_ADDRESS="$TARGET_TYPE.demo"

[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "Use an exact SDKv2 provider release"
# Do not send a real API token to a user-supplied alternative host.
[[ "$API_BASE" == "https://api.fastly.com" ]] ||
    fail "FASTLY_API_URL must be https://api.fastly.com"
[[ -n "${FASTLY_API_TOKEN:-}" ]] || fail "Missing required environment variable FASTLY_API_TOKEN"

if [[ "$SCENARIO" == fos ]]; then
    [[ -n "${FASTLY_FOS_TEST_BUCKET:-}" && -n "${FASTLY_FOS_TEST_REGION:-}" ]] ||
        fail "FOS requires FASTLY_FOS_TEST_BUCKET and FASTLY_FOS_TEST_REGION in a test-owned account"
fi
for cmd in go terraform jq curl cksum sed grep; do
    command -v "$cmd" >/dev/null || fail "Missing required command $cmd"
done

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-remaining-live.XXXXXX")"
PROJECT="$WORK_DIR/project"
PROVIDER="$WORK_DIR/provider"
MIGRATOR="$WORK_DIR/migrator"
SERVICE_NAME="tf-migrem-$$-$(date +%s)"
mkdir -p "$PROJECT" "$PROVIDER" "$WORK_DIR/backups"

# Secrets and state may appear in *.log, tfstate, saved plans and traces.
# Work directory is private (0700, all files 0600), never uploaded to CI.
export TF_IN_AUTOMATION=1 TF_INPUT=0
unset TF_LOG TF_LOG_PATH FASTLY_DEBUG_MODE TF_DATA_DIR TF_WORKSPACE TF_CLI_ARGS TF_CLI_ARGS_init TF_CLI_ARGS_plan TF_CLI_ARGS_apply TF_CLI_ARGS_destroy
export TF_VAR_test_service_name="$SERVICE_NAME"
if [[ "$SCENARIO" == fos ]]; then
    export TF_VAR_test_fos_bucket="$FASTLY_FOS_TEST_BUCKET"
    export TF_VAR_test_fos_region="$FASTLY_FOS_TEST_REGION"
fi

use_legacy() {
    export TF_CLI_CONFIG_FILE="$WORK_DIR/legacy.tfrc" FASTLY_API_KEY="$FASTLY_API_TOKEN"
}
use_target() {
    export TF_CLI_CONFIG_FILE="$WORK_DIR/target.tfrc"
    unset FASTLY_API_KEY
}
# Never echo Terraform stdout: it may include sensitive values.
quiet() {
    local label="$1"
    shift
    if ! "$@" > "$WORK_DIR/${label}.log" 2>&1; then
        fail "$label failed; inspect private log $WORK_DIR/${label}.log"
    fi
}
cleanup() {
    local result=$? original_phase="$PHASE"
    trap - EXIT INT TERM
    unset TF_LOG TF_LOG_PATH FASTLY_DEBUG_MODE
    if [[ "$CREATION_STARTED" == true ]]; then
        PHASE=cleanup
        # Use whichever provider matches the current configuration. May be
        # necessary even when initial apply failed part-way.
        if grep -Fq "resource \"$TARGET_TYPE\"" "$PROJECT/main.tf"; then
            use_target
        else
            use_legacy
        fi
        local destroy_ok=true
        if ! (cd "$PROJECT" && terraform destroy -auto-approve -input=false \
            > "$WORK_DIR/destroy.log" 2>&1); then
            destroy_ok=false
            printf '[WARN] Terraform destroy failed; checking the test-service API cleanup utility\n' >&2
        fi
        # A failed apply may create a service that is not tracked in Terraform
        # state, even when destroy returns success. On failed runs, verify
        # cleanup through the existing utility, limited to this unique name.
        if (( result != 0 )) || [[ "$destroy_ok" == false ]]; then
            if ! (cd "$ROOT" && go run ./scripts/cleanup-test-services \
                -prefix "$SERVICE_NAME" > "$WORK_DIR/cleanup-sweep.log" 2>&1); then
                printf '[ERROR] Test-service API cleanup failed for %s; inspect %s/cleanup-sweep.log and clean up manually\n' \
                    "$SERVICE_NAME" "$WORK_DIR" >&2
                result=1
            elif ! (cd "$ROOT" && go run ./scripts/cleanup-test-services \
                -prefix "$SERVICE_NAME" -dry-run > "$WORK_DIR/cleanup-verify.log" 2>&1) ||
                ! grep -Fxq "matched 0 service(s) with prefix \"$SERVICE_NAME\"" "$WORK_DIR/cleanup-verify.log"; then
                printf '[ERROR] Could not verify removal of test service %s; inspect %s/cleanup-verify.log\n' \
                    "$SERVICE_NAME" "$WORK_DIR" >&2
                result=1
            else
                info "No remaining test services found for $SERVICE_NAME"
            fi
        fi
    fi
    if (( result != 0 )); then
        printf '[ERROR] Live migration acceptance failed in phase %s\n' "$original_phase" >&2
    fi
    if (( result != 0 )) || [[ -n "${KEEP_WORK_DIR:-}" ]]; then
        printf '[INFO] PRIVATE work directory (contains secrets/state): %s\n' "$WORK_DIR"
    else
        rm -rf -- "$WORK_DIR"
    fi
    exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

PHASE=build
info "Building local Framework provider and migration CLI"
quiet build-provider bash -c 'cd "$1" && go build -o "$2/terraform-provider-fastly" .' _ "$ROOT" "$PROVIDER"
quiet build-migrator bash -c 'cd "$1" && go build -o "$2" ./cmd/fastly-tf-migrate' _ "$ROOT" "$MIGRATOR"
cat > "$WORK_DIR/legacy.tfrc" <<'TFRC'
provider_installation { direct {} }
TFRC
cat > "$WORK_DIR/target.tfrc" <<TFRC
provider_installation {
  dev_overrides { "fastly/fastly" = "$PROVIDER" }
  direct {}
}
TFRC
sed "s/@LEGACY_PROVIDER_VERSION@/$VERSION/g" \
    "$ROOT/scripts/test-migration-logging-remaining/live-fixtures/$SCENARIO/main.tf.tmpl" \
    > "$PROJECT/main.tf"

cd "$PROJECT"
PHASE=legacy-apply
use_legacy
info "Creating disposable SDKv2 $VERSION CDN service for $SCENARIO"
quiet legacy-init terraform init -input=false
terraform version -json | jq -e --arg v "$VERSION" \
    '.provider_selections["registry.terraform.io/fastly/fastly"] == $v' >/dev/null ||
    fail "Terraform did not select SDKv2 $VERSION"
# Reject invalid fixtures before a remote service creation can start.
quiet legacy-validate terraform validate
CREATION_STARTED=true
quiet legacy-apply terraform apply -input=false -auto-approve
quiet legacy-refresh terraform apply -input=false -auto-approve -refresh-only
quiet legacy-plan terraform plan -input=false -out="$WORK_DIR/legacy.tfplan"
terraform show -json "$WORK_DIR/legacy.tfplan" > "$WORK_DIR/legacy-plan.json"
jq -e '[.resource_changes[]? | select(.mode=="managed" and .change.actions != ["no-op"])] | length==0' \
    "$WORK_DIR/legacy-plan.json" >/dev/null ||
    fail "Legacy logging service is not stable before migration"

terraform show -json > "$WORK_DIR/legacy-state.json"
SERVICE_ID="$(jq -r --arg address "$LEGACY_ADDRESS" '.values.root_module.resources[]?
    | select(.address==$address) | .values.id' "$WORK_DIR/legacy-state.json")"
[[ -n "$SERVICE_ID" && "$SERVICE_ID" != null ]] || fail "Legacy service ID is absent"
jq -e --arg address "$LEGACY_ADDRESS" --arg id "$SERVICE_ID" \
    --arg endpoint "$LOGGING_BLOCK" --arg endpoint_name "$ENDPOINT_NAME" '
    [.values.root_module.resources[]?
       | select(.address==$address and .values.id==$id)
       | .[$endpoint][]? | select(.name==$endpoint_name)] | length==1
' "$WORK_DIR/legacy-state.json" >/dev/null ||
    fail "Legacy logging endpoint not found in state"

jq -eS --arg address "$LEGACY_ADDRESS" --arg id "$SERVICE_ID" \
    --arg endpoint "$LOGGING_BLOCK" \
    -f "$ROOT/scripts/test-migration-logging-remaining/live-fixtures/snapshot.jq"     "$WORK_DIR/legacy-state.json" > "$WORK_DIR/endpoint-before.json"

# Identity and active service version are checked against the live API,
# not solely against provider state or a plan snapshot.
snapshot_service() {
    local file="$1"
    curl --fail --silent --show-error \
        -H "Fastly-Key: $FASTLY_API_TOKEN" "$API_BASE/service/$SERVICE_ID" \
        > "$WORK_DIR/service-raw.json" ||
        fail "Cannot read the test service from the Fastly API"
    jq -eS --arg id "$SERVICE_ID" \
        '{id, name, active_version} | select(.id==$id)' \
        "$WORK_DIR/service-raw.json" > "$file" ||
        fail "Fastly API did not return the expected test service"
    rm -f "$WORK_DIR/service-raw.json"
}
snapshot_service "$WORK_DIR/service-before.json"

PHASE=migrate
info "Checking read-only preview and writing migration HCL"
before="$(cksum main.tf .terraform.lock.hcl)"
quiet analyze "$MIGRATOR" analyze .
quiet dry-run "$MIGRATOR" migrate --dry-run .
grep -Fxq 'Migration dry-run: READY' "$WORK_DIR/dry-run.log" ||
    fail "Migrator did not report a READY dry-run"
grep -Fq 'HCL diff withheld' "$WORK_DIR/dry-run.log" ||
    fail "Sensitive migration preview was not withheld"
[[ "$before" == "$(cksum main.tf .terraform.lock.hcl)" ]] ||
    fail "Migration preview modified source configuration"
lock="$(cksum .terraform.lock.hcl)"
quiet migrate-write "$MIGRATOR" migrate --write --backup-dir "$WORK_DIR/backups" .
[[ "$lock" == "$(cksum .terraform.lock.hcl)" ]] ||
    fail "Migration changed legacy dependency lockfile"
grep -Fq "resource \"$TARGET_TYPE\" \"demo\"" main.tf ||
    fail "Migrator did not rename service to CDN Auto"
grep -Fq "$LOGGING_BLOCK {" main.tf ||
    fail "Migrator removed the expected $SCENARIO logging endpoint"
# HCL formatting aligns the equals signs (for example, "to   = ...").
# Match both addresses while allowing Terraform's legal whitespace.
grep -Eq "^[[:space:]]*from[[:space:]]*=[[:space:]]*$LEGACY_TYPE[.]demo[[:space:]]*$" migration.tf ||
    fail "Expected Terraform moved block is missing"
grep -Eq "^[[:space:]]*to[[:space:]]*=[[:space:]]*$TARGET_TYPE[.]demo[[:space:]]*$" migration.tf ||
    fail "Expected Terraform move destination is missing"

PHASE=target-plan
info "Validating target configuration and auditing the migration-only plan"
use_target
# SDKv2 was initialized above. A dev override resolves the locally built
# target; do not re-run init because GA 20.x may not be published.
rm -f .terraform.lock.hcl
quiet target-validate terraform validate
TRACE="$WORK_DIR/provider-trace.log"
: > "$TRACE"
export FASTLY_DEBUG_MODE=true TF_LOG=TRACE TF_LOG_PATH="$TRACE"
quiet target-plan terraform plan -input=false -out="$WORK_DIR/migration.tfplan"
terraform show -json "$WORK_DIR/migration.tfplan" > "$WORK_DIR/migration-plan.json"
if ! jq -e '
  [.resource_changes[]?
   | select(.mode=="managed" and (
       (.change.actions != ["no-op"]) or (.change.importing != null)
     ))] | length==0
' "$WORK_DIR/migration-plan.json" >/dev/null; then
    # Report paths and actions only. Terraform plan JSON can contain plaintext
    # logging credentials: NEVER print .change.before or .change.after values.
    jq -r '
      .resource_changes[]?
      | select(.mode == "managed"
        and (.change.actions != ["no-op"] or .change.importing != null)) as $r
      | "  [PLAN] \($r.address) actions=\($r.change.actions | join(",")) previous=\($r.previous_address // "-")",
        ($r.change as $c
         | ([($c.before // {} | paths(scalars)),
             ($c.after // {} | paths(scalars))] | unique[]) as $p
         | select((try ($c.before | getpath($p)) catch null)
                  != (try ($c.after | getpath($p)) catch null))
         | "    changed path: \($p | map(tostring) | join("."))")
    ' "$WORK_DIR/migration-plan.json" >&2
    fail "Migration plan contains changes to Fastly resources; refusing to apply"
fi
jq -e --arg target "$TARGET_ADDRESS" --arg legacy "$LEGACY_ADDRESS" '
  [.resource_changes[]?
   | select(.address==$target
     and .previous_address==$legacy
     and .change.actions==["no-op"])] | length==1
' "$WORK_DIR/migration-plan.json" >/dev/null ||
    fail "Expected no-op cross-type service state move is missing"
info "Migration plan has zero managed-resource actions"

# Applying an approved no-op move persists the resource address in state.
# The saved plan contains no infrastructure changes; never apply a plan with
# create/update/delete/replace/import actions.
PHASE=target-apply
quiet target-apply terraform apply -input=false "$WORK_DIR/migration.tfplan"
PHASE=target-no-op
local_code=0
terraform plan -detailed-exitcode -input=false > "$WORK_DIR/final-plan.log" 2>&1 || local_code=$?
[[ "$local_code" == 0 ]] ||
    fail "Final Terraform plan is not a no-op (detailed exit code $local_code)"

terraform show -json > "$WORK_DIR/target-state.json"
jq -e --arg address "$TARGET_ADDRESS" --arg id "$SERVICE_ID" \
    --arg endpoint "$LOGGING_BLOCK" --arg endpoint_name "$ENDPOINT_NAME" '
    [.values.root_module.resources[]?
       | select(.address==$address and .values.id==$id)
       | .[$endpoint][]? | select(.name==$endpoint_name)] | length==1
' "$WORK_DIR/target-state.json" >/dev/null ||
    fail "Logging endpoint missing after state move"

jq -eS --arg address "$TARGET_ADDRESS" --arg id "$SERVICE_ID" \
    --arg endpoint "$LOGGING_BLOCK" \
    -f "$ROOT/scripts/test-migration-logging-remaining/live-fixtures/snapshot.jq"     "$WORK_DIR/target-state.json" > "$WORK_DIR/endpoint-after.json"
cmp -s "$WORK_DIR/endpoint-before.json" "$WORK_DIR/endpoint-after.json" ||
    fail "Logging endpoint state differs after migration (values withheld)"
snapshot_service "$WORK_DIR/service-after.json"
cmp -s "$WORK_DIR/service-before.json" "$WORK_DIR/service-after.json" ||
    fail "Fastly service identity, name or active version changed"

# Inspect only pre-cleanup migration operations. Destruction is allowed only
# in cleanup, which happens after these assertions.
calls="$(grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$TRACE" |
    sed -E 's/^.*"([A-Z]+ )/\1/' | sort -u || true)"
grep -q -E '^(GET|HEAD) /service/' <<< "$calls" ||
    fail "No service read calls recorded: provider API audit was not active"
unsafe="$(awk '$1 !~ /^(GET|HEAD)$/ && $2 ~ /^\/(service|enabled-products)(\/|$)/ {print}' <<< "$calls")"
[[ -z "$unsafe" ]] ||
    fail "Mutating service API calls occurred during state migration (details withheld)"
unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH

PHASE=final
info "$SCENARIO legacy/target endpoint state matches exactly"
info "Service ID and active version unchanged; zero managed-resource changes; final no-op"
info "Provider trace contains no mutating service/product API requests"
info "SDKv2 $VERSION CDN $SCENARIO logging live migration PASSED"
# Cleanup trap destroys the test-owned service. It may produce mutating API
# calls after the audit; those are not part of the migration itself.
