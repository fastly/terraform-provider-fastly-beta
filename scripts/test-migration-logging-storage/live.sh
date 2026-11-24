#!/usr/bin/env bash
# Opt-in, API-backed SDKv2 -> CDN/Compute Auto storage logging migration.
# Uses public, inert go-fastly test-fixture S3 credentials: this verifies
# Fastly endpoint configuration/state, NOT S3 object delivery. Creates only
# a disposable Fastly service and never touches existing customer services.
set -euo pipefail
umask 077

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${LEGACY_PROVIDER_VERSION:-10.3.0}"
API_BASE="${FASTLY_API_URL:-https://api.fastly.com}"
PHASE="preflight"
CREATION_STARTED=false
FAMILY="${1:-cdn}"
info() { printf '[INFO] %s\n' "$*"; }
fail() { printf '[ERROR] %s (phase: %s)\n' "$*" "$PHASE" >&2; exit 1; }
usage() {
    cat <<'HELP'
Opt-in live S3 logging migration acceptance test for a disposable CDN or Compute service.

Usage: live.sh [cdn|compute] (default: cdn)

Required environment:
  FASTLY_API_TOKEN              token authorized to create/delete test services
Optional:
  LEGACY_PROVIDER_VERSION       exact published SDKv2 version (default 10.3.0)
  KEEP_WORK_DIR=1               preserve private test files on success

This command CREATES and DESTROYS a real Fastly service. Use an isolated
Fastly test account; running the make target itself is the explicit opt-in.
S3 bucket and credentials are inert PUBLIC examples from go-fastly recordings
(no AWS account/bucket/keys required). The Fastly API may accept them as
configuration, but S3 delivery will NOT work and is NOT tested.
The harness rejects infrastructure-changing migration plans and checks
that the Fastly service identity, active version and S3 state are unchanged.
HELP
}
[[ "${1:-}" == "--help" ]] && { usage; exit 0; }
[[ $# -le 1 ]] || { usage >&2; exit 2; }
# Internal runner argument; the one public Makefile target runs both families.
case "$FAMILY" in
    cdn)
        LEGACY_TYPE=fastly_service_vcl
        TARGET_TYPE=fastly_service_cdn_auto
        FIXTURE_DIR=s3-cdn
        ;;
    compute)
        LEGACY_TYPE=fastly_service_compute
        TARGET_TYPE=fastly_service_compute_auto
        FIXTURE_DIR=s3-compute
        ;;
    *) printf '[ERROR] Usage: %s [cdn|compute]\n' "$0" >&2; exit 2 ;;
esac

LEGACY_ADDRESS="$LEGACY_TYPE.demo"
TARGET_ADDRESS="$TARGET_TYPE.demo"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "Use an exact SDKv2 provider release"
# Do not send a real API token to a user-supplied alternative host.
[[ "$API_BASE" == "https://api.fastly.com" ]] ||
    fail "FASTLY_API_URL must be https://api.fastly.com"
[[ -n "${FASTLY_API_TOKEN:-}" ]] || fail "Missing required environment variable FASTLY_API_TOKEN"

# Source-controlled PUBLIC AWS documentation examples; never require an S3
# receiver or accept external credentials. These are intentionally unusable
# for real S3 delivery and only exercise Fastly's logging configuration API.
# shellcheck source=live-fixtures/s3-cdn/test-values.sh
source "$ROOT/scripts/test-migration-logging-storage/live-fixtures/s3-cdn/test-values.sh"
[[ -n "$TEST_S3_BUCKET" && -n "$TEST_S3_ACCESS_KEY" && -n "$TEST_S3_SECRET_KEY" ]] ||
    fail "Missing source-controlled S3 test fixture values"
for cmd in go terraform jq curl cksum sed grep; do
    command -v "$cmd" >/dev/null || fail "Missing required command $cmd"
done

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-storage-live.XXXXXX")"
PROJECT="$WORK_DIR/project"
PROVIDER="$WORK_DIR/provider"
MIGRATOR="$WORK_DIR/migrator"
SERVICE_NAME="tf-migstor-$$-$(date +%s)"
mkdir -p "$PROJECT" "$PROVIDER" "$WORK_DIR/backups"

# Secrets and state may appear in *.log, tfstate, saved plans and traces.
# Work directory is private (0700, all files 0600), never uploaded to CI.
export TF_IN_AUTOMATION=1 TF_INPUT=0
unset TF_LOG TF_LOG_PATH FASTLY_DEBUG_MODE TF_DATA_DIR TF_WORKSPACE TF_CLI_ARGS TF_CLI_ARGS_init TF_CLI_ARGS_plan TF_CLI_ARGS_apply TF_CLI_ARGS_destroy
export TF_VAR_test_service_name="$SERVICE_NAME"
export TF_VAR_test_s3_bucket="$TEST_S3_BUCKET"
export TF_VAR_test_s3_access_key="$TEST_S3_ACCESS_KEY"
export TF_VAR_test_s3_secret_key="$TEST_S3_SECRET_KEY"
printf '%s' "$TEST_S3_ACCESS_KEY" > "$WORK_DIR/access.key"
printf '%s' "$TEST_S3_SECRET_KEY" > "$WORK_DIR/secret.key"

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
    "$ROOT/scripts/test-migration-logging-storage/live-fixtures/$FIXTURE_DIR/main.tf.tmpl" \
    > "$PROJECT/main.tf"
if [[ "$FAMILY" == compute ]]; then
    cp "$ROOT/internal/acceptance_tests/fixtures/packages/valid.tar.gz" "$PROJECT/package.tar.gz"
fi

cd "$PROJECT"
PHASE=legacy-apply
use_legacy
info "Creating disposable SDKv2 $VERSION $FAMILY service with dummy S3 fixture credentials"
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
    fail "Legacy S3 service is not stable before migration"

terraform show -json > "$WORK_DIR/legacy-state.json"
SERVICE_ID="$(jq -r --arg address "$LEGACY_ADDRESS" '.values.root_module.resources[]?
    | select(.address==$address) | .values.id' "$WORK_DIR/legacy-state.json")"
[[ -n "$SERVICE_ID" && "$SERVICE_ID" != null ]] || fail "Legacy service ID is absent"
jq -e --arg address "$LEGACY_ADDRESS" --arg id "$SERVICE_ID" --rawfile access "$WORK_DIR/access.key" \
    --rawfile secret "$WORK_DIR/secret.key" '
    [.values.root_module.resources[]?
      | select(.address==$address and .values.id==$id)
      | .values.logging_s3[]?
      | select(.name=="s3-live" and .s3_access_key==$access and .s3_secret_key==$secret)
    ] | length==1' "$WORK_DIR/legacy-state.json" >/dev/null ||
    fail "Legacy S3 credentials are missing or masked in state; cannot establish preservation"

# Normalize all shared S3 endpoint fields, not just the credential members.
# Any default/state divergence is a failure requiring review, not a pass.
jq -eS --arg address "$LEGACY_ADDRESS" --arg id "$SERVICE_ID" \
    --argjson is_legacy true -f "$ROOT/scripts/test-migration-logging-storage/live-fixtures/s3-cdn/snapshot.jq" "$WORK_DIR/legacy-state.json" \
    > "$WORK_DIR/endpoint-before.json"

# A Compute logging migration must also leave its package identity untouched.
# Compare only stable configured package fields (no provider-computed metadata).
snapshot_package() {
    local address="$1" state="$2" output="$3"
    jq -eS --arg address "$address" '
      [.values.root_module.resources[]?
       | select(.address==$address)
       | .values.package[]?
       | {filename, source_code_hash}]
      | if length == 1 then .[0] else error("expected one Compute package") end
    ' "$state" > "$output"
}
if [[ "$FAMILY" == compute ]]; then
    snapshot_package "$LEGACY_ADDRESS" "$WORK_DIR/legacy-state.json" "$WORK_DIR/package-before.json"
fi

# The API cannot be assumed to return credentials unredacted. The identity
# endpoint is deliberately a separate, direct check of the live service.
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
if grep -Ff "$WORK_DIR/access.key" "$WORK_DIR/dry-run.log" >/dev/null ||
    grep -Ff "$WORK_DIR/secret.key" "$WORK_DIR/dry-run.log" >/dev/null; then
    fail "Secret material appeared in migration preview"
fi
lock="$(cksum .terraform.lock.hcl)"
quiet migrate-write "$MIGRATOR" migrate --write --backup-dir "$WORK_DIR/backups" .
[[ "$lock" == "$(cksum .terraform.lock.hcl)" ]] ||
    fail "Migration changed legacy dependency lockfile"
grep -Fq "resource \"$TARGET_TYPE\" \"demo\"" main.tf ||
    fail "Migrator did not rename service to ${FAMILY} Auto"
grep -Fq 'authentication = {' main.tf ||
    fail "Migrator did not nest S3 authentication credentials"
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
jq -e --arg address "$TARGET_ADDRESS" --arg id "$SERVICE_ID" --rawfile access "$WORK_DIR/access.key" \
    --rawfile secret "$WORK_DIR/secret.key" '
    [.values.root_module.resources[]?
      | select(.address==$address and .values.id==$id)
      | .values.logging_s3[]?
      | select(.name=="s3-live"
          and .authentication.access_key==$access
          and .authentication.secret_key==$secret)
    ] | length==1' "$WORK_DIR/target-state.json" >/dev/null ||
    fail "S3 credentials were lost or masked after state move and refresh"
jq -eS --arg address "$TARGET_ADDRESS" --arg id "$SERVICE_ID" \
    --argjson is_legacy false -f "$ROOT/scripts/test-migration-logging-storage/live-fixtures/s3-cdn/snapshot.jq" "$WORK_DIR/target-state.json" \
    > "$WORK_DIR/endpoint-after.json"
cmp -s "$WORK_DIR/endpoint-before.json" "$WORK_DIR/endpoint-after.json" ||
    fail "S3 endpoint or credential state differs after migration (values withheld)"
if [[ "$FAMILY" == compute ]]; then
    snapshot_package "$TARGET_ADDRESS" "$WORK_DIR/target-state.json" "$WORK_DIR/package-after.json"
    cmp -s "$WORK_DIR/package-before.json" "$WORK_DIR/package-after.json" ||
        fail "Compute package state differs after migration (values withheld)"
fi
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
info "S3 legacy/target credentials and shared endpoint state match exactly"
info "Service ID and active version unchanged; zero managed-resource changes; final no-op"
info "Provider trace contains no mutating service/product API requests"
info "SDKv2 $VERSION $FAMILY storage logging live migration PASSED"
# Cleanup trap destroys the test-owned service. It may produce mutating API
# calls after the audit; those are not part of the migration itself.
