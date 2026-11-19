# VCL → CDN Auto state-migration acceptance test

Opt-in, real-service end-to-end test of the
`fastly_service_vcl` → `fastly_service_cdn_auto` migration
(**follow-up coverage for the completed CDTOOL-1854**).

This suite tests **only the VCL service state move**. Its fixture contains a
domain, backend, and `force_destroy = true`, with **no `reuse` argument or
`product_enablement` block**. The legacy provider rejects `force_destroy` and
`reuse` when both are explicitly configured. Product-split and product-import
acceptance tests belong in `scripts/test-migration-products/`; Compute tests
belong in the Compute migration follow-up (CDTOOL-1867).

The provider's existing
`internal/resources/servicecdnauto/state_migration_test.go` exercises the
`ResourceWithMoveState` implementation in isolation. This test covers the
previously missing Terraform Core / CLI / provider integration using an actual
Fastly service.

## Workflow and assertions

1. Build the CLI and the current dual-model provider from this checkout.
2. Create a throwaway legacy `fastly_service_vcl` with the published SDKv2
   `fastly/fastly` provider (exact version, default `10.3.0`) and perform a
   `terraform apply -refresh-only`.
3. Verify **persisted** legacy state schema version **1**, rather than
   assuming the provider version guarantees a compatible state schema.
4. Snapshot the live Fastly service ID, name, and active version.
5. Run `fastly-tf-migrate analyze`, `migrate --dry-run`, and
   `migrate --write --backup-dir`. Verify dry-run does not change files,
   `fastly_service_cdn_auto` HCL and a correct Terraform `moved` block
   are generated, and a private recovery manifest is created.
6. Reject any generated product resource or `import` block.
7. Switch to the locally compiled dual-model provider using a
   `fastly/fastly` Terraform development override, then run
   `terraform validate` and a **refresh-enabled** `terraform plan`.
8. Require the plan to report `previous_address = fastly_service_vcl.demo`
   for `fastly_service_cdn_auto.demo`, **zero imports**, and **no
   create/update/delete/replace** actions.
9. Apply the saved plan and require a final
   `terraform plan -detailed-exitcode` with exit code **0**.
10. Verify only the new resource address remains in state, the **original
    Fastly service ID** and explicit `force_destroy` setting are preserved,
    `reuse` is checked if present in legacy state, and the migration CLI is
    idempotent on rerun.
11. Compare the live service snapshot before and after and examine the
    provider's HTTP trace for unintended mutating service API calls.
12. Destroy the test service.

On failure, the script attempts `terraform destroy` followed by the existing
`cleanup-test-services` name-prefix sweep. **Check the output for cleanup
warnings** rather than assuming deletion succeeded.

## Run

Run against a **non-production Fastly test account**:

```bash
export FASTLY_API_TOKEN=...
make test-migration-vcl

# Or directly:
./scripts/test-migration-vcl/run.sh
```

The script takes **no scenario argument** and requires no optional Fastly
product entitlement. It needs `terraform` (1.8+), `go`, `jq`, `curl`, and
`cksum`, plus network access to Fastly and the legacy provider registry.

Optional environment variables:

- `LEGACY_PROVIDER_VERSION`: exact published legacy provider release, default
  `10.3.0`. The script verifies the actual selected version after `terraform init`.
  For historical regression coverage, run with `9.1.0`, `9.5.0`, `9.8.0`,
  `10.2.0`, and `10.3.0`. Each invocation creates a separate live test service.
  The credential-free published-schema check in
  `scripts/test-migration-state-schemas/` complements these live tests.
- `FASTLY_API_URL`: API base URL, default `https://api.fastly.com`.
- `KEEP_WORK_DIR=1`: retain temporary files for debugging.

**Security:** The private work directory holds Terraform state, HCL backups,
plan JSON, and HTTP trace logs that may contain sensitive data. It is removed
after a successful run unless `KEEP_WORK_DIR` is set. Failed runs retain
the directory to facilitate debugging. Do not commit or upload its contents
without inspecting and redacting secrets.

## Development override versus released GA provider

This is a **pre-GA** acceptance test. The dual-model provider currently
builds from the `fastly/fastly-beta` repository, but the migrator targets
the future GA provider address, `fastly/fastly`. As in the existing
product-migration suite, the script uses a local development override so
Terraform exercises the current dual-model implementation under that target
address. It skips `terraform init` after the rewrite while no registry
provider matching `>= 20.0.0` is available.

When GA is published, add or adapt a registry-based test path using
`terraform init -upgrade` after the rewrite and account for any legacy
provider source rename.

This live test is **opt-in**. It is deliberately not part of `test-unit`,
`test-migrator`, or `release-check` because it creates and deletes live
Fastly services.
