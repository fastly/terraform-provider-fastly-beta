#!/usr/bin/env bash
# Validates migrated traffic-block test configurations against the target provider schema.
# No init, state or API calls.
set -euo pipefail
umask 077
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-traffic-config.XXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
unset TF_DATA_DIR TF_WORKSPACE TF_CLI_ARGS TF_CLI_ARGS_validate TF_LOG TF_LOG_PATH FASTLY_DEBUG_MODE
mkdir -p "$WORK_DIR/provider"
(
    cd "$REPO_ROOT"
    go build -o "$WORK_DIR/provider/terraform-provider-fastly" .
    go build -o "$WORK_DIR/migrator" ./cmd/fastly-tf-migrate
)
cat > "$WORK_DIR/override.tfrc" <<TFRC
provider_installation {
  dev_overrides {
    "fastly/fastly" = "$WORK_DIR/provider"
  }
  direct {}
}
TFRC
export TF_CLI_CONFIG_FILE="$WORK_DIR/override.tfrc"
FIXTURES="$REPO_ROOT/internal/migration/migrate/testdata/traffic"
cat > "$WORK_DIR/legacy.lock.hcl" <<'HCL'
provider "registry.terraform.io/fastly/fastly" {
  version = "10.3.0"
}
HCL
for fixture in "$FIXTURES"/*/; do
    name="$(basename "$fixture")"
    mkdir -p "$WORK_DIR/$name"
    cd "$WORK_DIR/$name"
    cp "$fixture/legacy.tf" main.tf
    cp "$WORK_DIR/legacy.lock.hcl" .terraform.lock.hcl
    "$WORK_DIR/migrator" migrate --write --backup-dir "$WORK_DIR/backup-$name" .
    cmp main.tf "$fixture/expected.tf"
    cmp .terraform.lock.hcl "$WORK_DIR/legacy.lock.hcl"
    rm -- .terraform.lock.hcl
    # The published S3 example also declares AWS resources; validate only the Fastly config.
    if grep -q 'resource "aws_' main.tf; then
        awk '/^resource "aws_/{skip=1} skip&&/^}/{skip=0;next} !skip' main.tf > main.tf.fastly && mv main.tf.fastly main.tf
    fi
    terraform validate
    echo "[INFO] $name: migrated configuration is valid"
done
