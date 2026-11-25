#!/usr/bin/env bash
# Target provider schema validation of synthetic HCL. No init, state or API.
set -euo pipefail
umask 077
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-remaining-config.XXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
unset TF_DATA_DIR TF_WORKSPACE TF_CLI_ARGS TF_CLI_ARGS_validate TF_LOG TF_LOG_PATH FASTLY_DEBUG_MODE
# Terraform validation must not inherit real Fastly credentials or endpoint
# environment defaults from the developer's shell. This is a subprocess.
while IFS='=' read -r name _; do
    case "$name" in
        FASTLY_*) unset "$name" ;;
    esac
done < <(env)
mkdir -p "$WORK_DIR/provider" "$WORK_DIR/cdn"
(
    cd "$REPO_ROOT"
    go build -o "$WORK_DIR/provider/terraform-provider-fastly" .
    go build -o "$WORK_DIR/migrator" ./cmd/fastly-tf-migrate
)
cat > "$WORK_DIR/override.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "fastly/fastly" = "$WORK_DIR/provider"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$WORK_DIR/override.tfrc"
FIXTURES="$REPO_ROOT/internal/migration/migrate/testdata/logging_remaining"
# Match the synthetic source-version fixture used by migration unit tests.
# This lockfile is only a test input to the analyzer, not a provider install.
cat > "$WORK_DIR/legacy.lock.hcl" <<'HCL'
provider "registry.terraform.io/fastly/fastly" {
  version = "10.3.0"
}
HCL
for family in cdn; do
    cd "$WORK_DIR/$family"
    cp "$FIXTURES/$family/legacy.tf" main.tf
    cp "$WORK_DIR/legacy.lock.hcl" .terraform.lock.hcl
    "$WORK_DIR/migrator" migrate --write --backup-dir "$WORK_DIR/backup-$family" .
    cmp main.tf "$FIXTURES/$family/expected.tf"
    # Migration must not alter the selected source provider lockfile.
    cmp .terraform.lock.hcl "$WORK_DIR/legacy.lock.hcl"
    # The synthetic legacy lockfile is not a valid target-provider selection.
    # Only discard it here, in the disposable fixture, before validation.
    rm -- .terraform.lock.hcl
    # dev_overrides uses the locally built GA provider; registry 20.x is not
    # required, and terraform init would try to resolve it pre-GA.
    terraform validate
    echo "[INFO] $family combined remaining logging migration golden HCL is target-valid"
done
