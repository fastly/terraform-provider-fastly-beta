#!/usr/bin/env bash
#
# Opt-in live NGWAF migration. Creates one object for every renamed or split
# NGWAF target with an exact SDKv2 release, then moves all of them. By default
# the target HCL and moved blocks are hand-written, which tests only the
# provider's state moves. NGWAF_MIGRATION_MODE=cli has fastly-tf-migrate write
# them instead, testing the full CLI workflow.
set -euo pipefail
umask 077
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
VERSION="${LEGACY_PROVIDER_VERSION:-10.3.0}"
VIRTUAL_PATCH_ID="${NGWAF_VIRTUAL_PATCH_ID:-CVE-2017-5638}"
MODE="${NGWAF_MIGRATION_MODE:-manual}"

[[ $# -eq 0 ]] || { echo "Usage: $0" >&2; exit 1; }
[[ -n "${FASTLY_API_TOKEN:-}" ]] || { echo "Set FASTLY_API_TOKEN for an isolated test account" >&2; exit 1; }
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Use an exact legacy provider version" >&2; exit 1; }
[[ "$MODE" == manual || "$MODE" == cli ]] || { echo "NGWAF_MIGRATION_MODE must be manual or cli" >&2; exit 1; }
for cmd in terraform go jq cksum; do
    command -v "$cmd" >/dev/null 2>&1 || { echo "Missing prerequisite: $cmd" >&2; exit 1; }
done

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-move-ngwaf.XXXXXX")"
PROJECT="$WORK_DIR/project"
# List names allow at most 32 characters and signal names no underscores.
PREFIX="tfmig-$$-$(date +%s | tail -c 6)"
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
    if (( status != 0 )) && [[ -f "$PROJECT/terraform.tfstate" ]]; then
        printf '[ERROR] Failed in phase %s (exit %s)\n' "$PHASE" "$status" >&2
        # Destroy with whichever provider owns the current state addresses.
        local addresses
        addresses="$(cd "$PROJECT" && terraform state list 2>/dev/null || true)"
        if grep -Fxq -f <(printf '%s\n' "${MOVES[@]}") <<< "$addresses"; then
            use_target
            write_target_config
        else
            use_legacy
            write_legacy_config
            rm -f "$PROJECT/migration.tf"
        fi
        (cd "$PROJECT" && terraform destroy -auto-approve -input=false) ||
            info "Cleanup failed; remove NGWAF objects named $PREFIX* (account lists, rules and signals are account-wide)"
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
MIGRATOR="$WORK_DIR/fastly-tf-migrate"
if [[ "$MODE" == cli ]]; then
    (cd "$REPO_ROOT" && go build -o "$MIGRATOR" ./cmd/fastly-tf-migrate)
fi
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

LIST_TYPES=(ip string country wildcard signal)
declare -A LIST_ENTRY=([ip]="10.0.0.1" [string]="admin" [country]="US" [wildcard]="admin-*" [signal]="XSS")
ALERTS=(datadog jira mailing_list microsoft_teams opsgenie pagerduty slack webhook)
declare -A ALERT_SECRET=(
    [datadog]='key = "1234567890abcdef1234567890abcdef"'
    [jira]='key = "1234567890abcdef1234567890abcdef"'
    [mailing_list]='address = "alerts@example.com"'
    [microsoft_teams]='webhook = "https://example.com/webhooks/my-service"'
    [opsgenie]='key = "1234567890abcdef1234567890abcdef"'
    [pagerduty]='key = "1234567890abcdef1234567890abcdef"'
    [slack]='webhook = "https://example.com/webhooks/my-service"'
    [webhook]='webhook = "https://example.com/webhooks/my-service"'
)
declare -A ALERT_EXTRA=(
    [datadog]='site = "us1"'
    [jira]=$'host       = "https://example.atlassian.net"\n  username   = "terraform"\n  project    = "TF"\n  issue_type = "Task"'
)

# The workspace and workspace signal keep their names, so both configs share them.
# Thresholds are explicit: legacy and target attack_signal_thresholds defaults differ.
shared_config() {
    local requirement="$1"
    cat <<EOF
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "$requirement"
    }
  }
}

provider "fastly" {}

resource "fastly_ngwaf_workspace" "demo" {
  name        = "$PREFIX"
  description = "Legacy NGWAF state move test"
  mode        = "log"

  attack_signal_thresholds {
    one_minute  = 100
    ten_minutes = 500
    one_hour    = 2000
    immediate   = false
  }
}

resource "fastly_ngwaf_workspace_signal" "demo" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  name         = "$PREFIX signal"
  description  = "Rate limit signal"
}

EOF
}

write_legacy_config() {
    {
        shared_config "= $VERSION"
        for type in "${LIST_TYPES[@]}"; do
            cat <<EOF
resource "fastly_ngwaf_account_list" "$type" {
  name        = "$PREFIX-$type"
  description = "Account $type list"
  type        = "$type"
  entries     = ["${LIST_ENTRY[$type]}"]
}

resource "fastly_ngwaf_workspace_list" "$type" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  name         = "$PREFIX-ws-$type"
  description  = "Workspace $type list"
  type         = "$type"
  entries      = ["${LIST_ENTRY[$type]}"]
}

EOF
        done
        for alert in "${ALERTS[@]}"; do
            cat <<EOF
resource "fastly_ngwaf_alert_${alert}_integration" "demo" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  description  = "$alert alert"
  ${ALERT_SECRET[$alert]}
  ${ALERT_EXTRA[$alert]:-}
}

EOF
        done
        cat <<EOF
resource "fastly_ngwaf_account_rule" "request" {
  applies_to  = [fastly_ngwaf_workspace.demo.id]
  type        = "request"
  description = "Account request rule"
  enabled     = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "127.0.0.1"
  }

  action {
    type = "block"
  }
}

resource "fastly_ngwaf_account_rule" "signal" {
  applies_to  = [fastly_ngwaf_workspace.demo.id]
  type        = "signal"
  description = "Account signal rule"
  enabled     = true

  condition {
    field    = "path"
    operator = "like"
    value    = "/contact-form"
  }

  action {
    type   = "exclude_signal"
    signal = "XSS"
  }
}

resource "fastly_ngwaf_workspace_rule" "request" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  type         = "request"
  description  = "Workspace request rule"
  enabled      = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "127.0.0.1"
  }

  action {
    type = "block"
  }
}

resource "fastly_ngwaf_workspace_rule" "signal" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  type         = "signal"
  description  = "Workspace signal rule"
  enabled      = true

  condition {
    field    = "path"
    operator = "like"
    value    = "/contact-form"
  }

  action {
    type   = "exclude_signal"
    signal = "XSS"
  }
}

resource "fastly_ngwaf_workspace_rule" "rate_limit" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  type         = "rate_limit"
  description  = "Workspace rate limit rule"
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  rate_limit {
    duration  = 300
    interval  = 60
    signal    = fastly_ngwaf_workspace_signal.demo.reference_id
    threshold = 100

    client_identifiers {
      type = "ip"
    }
  }

  action {
    type   = "log_request"
    signal = fastly_ngwaf_workspace_signal.demo.reference_id
  }
}

resource "fastly_ngwaf_workspace_rule" "templated_signal" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  type         = "templated_signal"
  description  = ""
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  action {
    type   = "templated_signal"
    signal = "LOGINATTEMPT"
  }
}

resource "fastly_ngwaf_account_signal" "demo" {
  applies_to  = ["*"]
  name        = "$PREFIX account signal"
  description = "Account signal"
}

resource "fastly_ngwaf_redaction" "demo" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  field        = "password"
  type         = "request_parameter"
}

resource "fastly_ngwaf_thresholds" "demo" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  name         = "$PREFIX-threshold"
  action       = "block"
  dont_notify  = false
  duration     = 86400
  enabled      = true
  interval     = 3600
  limit        = 10
  signal       = "SQLI"
}

resource "fastly_ngwaf_virtual_patches" "demo" {
  workspace_id     = fastly_ngwaf_workspace.demo.id
  virtual_patch_id = "$VIRTUAL_PATCH_ID"
  action           = "block"
  enabled          = true
}
EOF
    } > "$PROJECT/main.tf"
}

write_target_config() {
    {
        shared_config ">= 20.0.0"
        for type in "${LIST_TYPES[@]}"; do
            cat <<EOF
resource "fastly_ngwaf_${type}_list" "$type" {
  name        = "$PREFIX-$type"
  description = "Account $type list"
  entries     = ["${LIST_ENTRY[$type]}"]
}

resource "fastly_ngwaf_workspace_${type}_list" "$type" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  name         = "$PREFIX-ws-$type"
  description  = "Workspace $type list"
  entries      = ["${LIST_ENTRY[$type]}"]
}

EOF
        done
        for alert in "${ALERTS[@]}"; do
            cat <<EOF
resource "fastly_ngwaf_workspace_alert_${alert}_integration" "demo" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  description  = "$alert alert"
  ${ALERT_EXTRA[$alert]:-}

  authentication = {
    ${ALERT_SECRET[$alert]}
  }
}

EOF
        done
        cat <<EOF
resource "fastly_ngwaf_request_rule" "request" {
  applies_to  = [fastly_ngwaf_workspace.demo.id]
  description = "Account request rule"
  enabled     = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "127.0.0.1"
  }

  action {
    type = "block"
  }
}

resource "fastly_ngwaf_signal_rule" "signal" {
  applies_to  = [fastly_ngwaf_workspace.demo.id]
  description = "Account signal rule"
  enabled     = true

  condition {
    field    = "path"
    operator = "like"
    value    = "/contact-form"
  }

  action {
    signal = "XSS"
  }
}

resource "fastly_ngwaf_workspace_request_rule" "request" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  description  = "Workspace request rule"
  enabled      = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "127.0.0.1"
  }

  action {
    type = "block"
  }
}

resource "fastly_ngwaf_workspace_signal_rule" "signal" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  description  = "Workspace signal rule"
  enabled      = true

  condition {
    field    = "path"
    operator = "like"
    value    = "/contact-form"
  }

  action {
    signal = "XSS"
  }
}

resource "fastly_ngwaf_workspace_rate_limit_rule" "rate_limit" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  description  = "Workspace rate limit rule"
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  rate_limit {
    duration  = 300
    interval  = 60
    signal    = fastly_ngwaf_workspace_signal.demo.reference_id
    threshold = 100

    client_identifiers {
      type = "ip"
    }
  }

  action {
    type   = "log_request"
    signal = fastly_ngwaf_workspace_signal.demo.reference_id
  }
}

resource "fastly_ngwaf_workspace_templated_signal_rule" "templated_signal" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  action {
    signal = "LOGINATTEMPT"
  }
}

resource "fastly_ngwaf_signal" "demo" {
  applies_to  = ["*"]
  name        = "$PREFIX account signal"
  description = "Account signal"
}

resource "fastly_ngwaf_workspace_redaction" "demo" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  field        = "password"
  type         = "request_parameter"
}

resource "fastly_ngwaf_workspace_threshold" "demo" {
  workspace_id = fastly_ngwaf_workspace.demo.id
  name         = "$PREFIX-threshold"
  action       = "block"
  dont_notify  = false
  duration     = 86400
  enabled      = true
  interval     = 3600
  limit        = 10
  signal       = "SQLI"
}

resource "fastly_ngwaf_workspace_virtual_patch" "demo" {
  workspace_id     = fastly_ngwaf_workspace.demo.id
  virtual_patch_id = "$VIRTUAL_PATCH_ID"
  mode             = "block"
  enabled          = true
}
EOF
    } > "$PROJECT/main.tf"
}

# legacy address => target address, for every renamed or split resource.
declare -A MOVES=(
    [fastly_ngwaf_account_rule.request]=fastly_ngwaf_request_rule.request
    [fastly_ngwaf_account_rule.signal]=fastly_ngwaf_signal_rule.signal
    [fastly_ngwaf_workspace_rule.request]=fastly_ngwaf_workspace_request_rule.request
    [fastly_ngwaf_workspace_rule.signal]=fastly_ngwaf_workspace_signal_rule.signal
    [fastly_ngwaf_workspace_rule.rate_limit]=fastly_ngwaf_workspace_rate_limit_rule.rate_limit
    [fastly_ngwaf_workspace_rule.templated_signal]=fastly_ngwaf_workspace_templated_signal_rule.templated_signal
    [fastly_ngwaf_account_signal.demo]=fastly_ngwaf_signal.demo
    [fastly_ngwaf_redaction.demo]=fastly_ngwaf_workspace_redaction.demo
    [fastly_ngwaf_thresholds.demo]=fastly_ngwaf_workspace_threshold.demo
    [fastly_ngwaf_virtual_patches.demo]=fastly_ngwaf_workspace_virtual_patch.demo
)
for type in "${LIST_TYPES[@]}"; do
    MOVES[fastly_ngwaf_account_list.$type]=fastly_ngwaf_${type}_list.$type
    MOVES[fastly_ngwaf_workspace_list.$type]=fastly_ngwaf_workspace_${type}_list.$type
done
for alert in "${ALERTS[@]}"; do
    MOVES[fastly_ngwaf_alert_${alert}_integration.demo]=fastly_ngwaf_workspace_alert_${alert}_integration.demo
done
(( ${#MOVES[@]} == 28 )) || fail "Expected 28 moves, got ${#MOVES[@]}"

PHASE="legacy-apply"
cd "$PROJECT"
use_legacy
write_legacy_config
info "Creating live NGWAF objects with exact SDKv2 version $VERSION"
terraform init -backend=false -input=false
terraform version -json | jq -e --arg version "$VERSION" \
    '.provider_selections["registry.terraform.io/fastly/fastly"] == $version' >/dev/null ||
    fail "Terraform did not select exact SDKv2 version $VERSION"
terraform apply -auto-approve -input=false
terraform apply -refresh-only -auto-approve -input=false

# Record every legacy ID from actual state, keyed by the target address.
moves_json="$(for from in "${!MOVES[@]}"; do printf '%s\t%s\n' "$from" "${MOVES[$from]}"; done |
    jq -R -s 'split("\n") | map(select(length > 0) | split("\t") | {key: .[0], value: .[1]}) | from_entries')"
terraform state pull | jq -e --argjson moves "$moves_json" '
  [.resources[]? | select(.mode == "managed") | (.type + "." + .name) as $addr
   | select($moves[$addr] != null) | .instances[]?
   | select(.schema_version == 0 and (.attributes.id | length) > 0)
   | {key: $moves[$addr], value: {id: .attributes.id, workspace_id: .attributes.workspace_id}}]
  | if length == 28 then from_entries else error("expected 28 legacy objects at schema version 0") end
' > "$WORK_DIR/before.json" || fail "Legacy persisted NGWAF state is incomplete or not schema version 0"

if [[ "$MODE" == cli ]]; then
    PHASE="cli-migrate"
    "$MIGRATOR" analyze .
    before="$(cksum main.tf)"
    "$MIGRATOR" migrate --dry-run . > "$WORK_DIR/dry-run.txt"
    grep -Fxq 'Migration dry-run: READY' "$WORK_DIR/dry-run.txt" || fail "Migration dry-run was not READY"
    grep -Fxq 'Resources to move: 28' "$WORK_DIR/dry-run.txt" || fail "Expected 28 NGWAF resource moves"
    [[ "$before" == "$(cksum main.tf)" && ! -e migration.tf ]] || fail "Dry-run modified Terraform configuration"
    "$MIGRATOR" migrate --write --backup-dir "$WORK_DIR/backups" .
    for from in "${!MOVES[@]}"; do
        grep -Fq "from = $from" migration.tf || fail "migration.tf has no moved block for $from"
        grep -Fq "resource \"${MOVES[$from]%%.*}\" \"${MOVES[$from]#*.}\"" main.tf ||
            fail "main.tf does not declare ${MOVES[$from]}"
    done
    ! grep -Eq '^resource "fastly_ngwaf_(account_(list|rule|signal)|workspace_(list|rule)|redaction|thresholds|virtual_patches|alert_[a-z_]+)" ' main.tf ||
        fail "main.tf still declares a legacy NGWAF resource type"
    ! grep -Fq "= $VERSION" main.tf migration.tf || fail "Provider constraint was not raised to the GA provider"
else
    PHASE="manual-state-move"
    write_target_config
    for from in "${!MOVES[@]}"; do
        printf 'moved {\n  from = %s\n  to   = %s\n}\n\n' "$from" "${MOVES[$from]}"
    done > migration.tf
fi
use_target
rm -f .terraform.lock.hcl
terraform validate

PHASE="migration-plan"
TRACE="$WORK_DIR/provider-trace.log"
export FASTLY_DEBUG_MODE=true TF_LOG=TRACE TF_LOG_PATH="$TRACE"
terraform plan -input=false -out="$WORK_DIR/move.tfplan"
terraform show -json "$WORK_DIR/move.tfplan" > "$WORK_DIR/plan.json"
jq -e --argjson moves "$moves_json" '
  all(.resource_changes[]?; all(.change.actions[]; . == "no-op" or . == "read"))
  and ([.resource_changes[]? | select(.previous_address != null
        and $moves[.previous_address] == .address and .change.importing == null)] | length == 28)
' "$WORK_DIR/plan.json" >/dev/null || {
    jq -r '.resource_changes[]? | select(any(.change.actions[]; . != "no-op" and . != "read"))
           | "\(.address): \(.change.actions | join(","))"' "$WORK_DIR/plan.json" >&2
    fail "NGWAF migration plan did not report 28 state-only moves"
}

PHASE="migration-apply"
terraform apply -input=false "$WORK_DIR/move.tfplan"
code=0
terraform plan -input=false -detailed-exitcode || code=$?
[[ "$code" -eq 0 ]] || fail "Final NGWAF plan was not a no-op (exit $code)"
unset FASTLY_DEBUG_MODE TF_LOG TF_LOG_PATH

# Audit provider traffic: the move must not write to any NGWAF object.
calls="$(grep -a -E -o 'http\.Request \(dump\): \\?"[A-Z]+ [^ ]+' "$TRACE" |
    sed -E 's/^.*"([A-Z]+ )/\1/' | sort -u || true)"
grep -q -E '^GET /ngwaf/' <<< "$calls" || fail "Provider HTTP audit captured no NGWAF reads"
unsafe="$(awk '$1 !~ /^(GET|HEAD)$/ && $2 ~ /^\/ngwaf\// {print}' <<< "$calls")"
[[ -z "$unsafe" ]] || fail "Unexpected mutating NGWAF API requests: $unsafe"

PHASE="verify-state"
terraform state list > "$WORK_DIR/addresses.txt"
for from in "${!MOVES[@]}"; do
    grep -Fxq "${MOVES[$from]}" "$WORK_DIR/addresses.txt" || fail "Target state missing for ${MOVES[$from]}"
    ! grep -Fxq "$from" "$WORK_DIR/addresses.txt" || fail "Legacy state still present for $from"
done
terraform show -json | jq -e --slurpfile before "$WORK_DIR/before.json" '
  [.values.root_module.resources[]? | select($before[0][.address] != null)
   | {key: .address, value: {id: .values.id, workspace_id: .values.workspace_id}}]
  | from_entries == $before[0]
' >/dev/null || fail "Migration changed an NGWAF object ID or workspace"

if [[ "$MODE" == cli ]]; then
    PHASE="cli-idempotency"
    "$MIGRATOR" migrate --dry-run . > "$WORK_DIR/dry-run-after.txt"
    grep -Fxq 'Resources to move: 0' "$WORK_DIR/dry-run-after.txt" || fail "Repeated migration still proposes a move"
    grep -Fxq 'Files to change: 0' "$WORK_DIR/dry-run-after.txt" || fail "Repeated migration is not idempotent"
fi

PHASE="destroy"
terraform destroy -auto-approve -input=false
info "NGWAF $MODE migration of 28 resources from SDKv2 $VERSION PASSED"
