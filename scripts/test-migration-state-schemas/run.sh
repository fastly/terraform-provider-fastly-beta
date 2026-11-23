#!/usr/bin/env bash
#
# Verify decoder compatibility with independently exported SDKv2 provider
# schemas. This downloads pinned registry packages but creates no resources
# and requires no Fastly API token.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tf-migrate-schemas.XXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

for cmd in terraform go jq; do
    command -v "$cmd" >/dev/null 2>&1 || { echo "Missing prerequisite: $cmd" >&2; exit 1; }
done

# Do not accidentally export the local beta provider at a legacy version.
# Avoid ambient Terraform arguments/workspaces, and never use a user's backend.
unset TF_CLI_ARGS TF_CLI_ARGS_init TF_CLI_ARGS_version TF_CLI_ARGS_providers TF_DATA_DIR TF_WORKSPACE
export TF_CLI_CONFIG_FILE="$WORK_DIR/terraform.rc"
cat > "$TF_CLI_CONFIG_FILE" <<'EOF'
provider_installation {
  direct {}
}
EOF

for version in 9.1.0 9.5.0 9.8.0 10.2.0 10.3.0; do
    project="$WORK_DIR/$version"
    mkdir -p "$project"
    cat > "$project/main.tf" <<EOF
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "= $version"
    }
  }
}
EOF
    printf '\n[INFO] Exporting published SDKv2 schema from %s\n' "$version"
    (
        cd "$project"
        terraform init -backend=false -input=false -no-color
        terraform version -json | jq -e --arg version "$version" '
          .provider_selections["registry.terraform.io/fastly/fastly"] == $version
        ' >/dev/null
        terraform providers schema -json > "$WORK_DIR/$version.json"
    )
done

cd "$REPO_ROOT"
FASTLY_LEGACY_SCHEMA_DIR="$WORK_DIR" go test -count=1 -v ./internal/provider \
    -run '^(TestLegacyServiceMoveState(PublishedSchemas|HistoricalSchemas|AbsentAndNullValues|ComputePackageInputs|RejectsUnsafeJSON)|TestLegacyDomainPublished(Schemas|SchemaStateConversion)|TestLegacyNGWAFMoveState.*|TestLegacyContainerMoveStatePublishedSchemas)$'
