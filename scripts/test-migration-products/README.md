# Configurable product migration end-to-end test

Exercises `fastly-tf-migrate` against live Fastly resources for Bot Management,
DDoS Protection and NGWAF with non-default settings (plus WebSockets).

## Flow

1. Create a throwaway NGWAF workspace (`mode: off`) via the API, or use the one in
   `FASTLY_TEST_NGWAF_WORKSPACE_ID` (never deleted in that case).
2. Apply a legacy `fastly_service_vcl` with `product_enablement` using the
   published legacy `fastly/fastly` provider, then `apply -refresh-only`.
3. `fastly-tf-migrate analyze`, `migrate --dry-run`, `migrate --write`; assert the
   legacy block is gone and the settings appear in `migration.tf`.
4. Switch to the locally built provider (served via `dev_overrides` for
   `fastly/fastly`; `terraform init` is skipped because no 20.x release exists on
   the registry), plan, and assert 4 imports with no creates or deletes.
5. Apply, then require a no-op `plan -detailed-exitcode`.
6. Confirm the products are still enabled through the enabled-products API.
7. Destroy the service and delete the workspace. On failure, an emergency
   destroy and a name-prefix API sweep run.

## Running

```bash
export FASTLY_API_TOKEN=...
./scripts/test-migration-products/run.sh
```

Requires `terraform`, `go`, `jq` and `curl`. The account needs Bot Management,
DDoS Protection and NGWAF entitlements. Set `LEGACY_PROVIDER_VERSION` to change
the legacy provider floor (default `9.5.0`) and `KEEP_WORK_DIR=1` to keep the
temporary directory after a successful run.

## Changes required when `fastly/fastly` 20.0.0 is released

The script currently works around the GA provider not being on the registry.
When 20.0.0 is published, update `run.sh` so it tests what users will actually do:

- **Install the GA provider from the registry.** Remove the `dev_overrides` file
  written in `build_binaries` and the `go build -o .../terraform-provider-fastly`
  step. The provider build is then only needed for pre-release testing.
- **Run `terraform init -upgrade` after migrating.** `plan_and_apply_ga` currently
  skips `init` because the migrator's `>= 20.0.0` constraint cannot be resolved
  without a release. Restore `terraform init -upgrade -input=false` and drop the
  `rm -f .terraform.lock.hcl` workaround, so the stale legacy lock entry is
  replaced by a real upgrade. This is the documented user flow.
- **Stop swapping environment variables by hand.** `use_legacy_provider` and
  `use_ga_provider` exist because the legacy provider reads `FASTLY_API_KEY` and
  the GA provider reads `FASTLY_API_TOKEN`. Keep that split, since it mirrors the
  documented migration step, but drop the `TF_CLI_CONFIG_FILE` handling.
- **Revisit the legacy version floor.** `LEGACY_PROVIDER_VERSION` (default
  `9.5.0`) resolves to the newest release below 20, which is 10.2.0 today. Decide
  whether to pin a specific version or test both the 9.1.0 floor and the latest.
- **Wire the test into release verification.** It is not part of `make` targets or
  CI. Consider adding a `make test-migration` target and including it in
  `release-check`, which needs the account entitlements listed under Running.
- **Update the legacy source name if the rename ships.** The migrator README says
  the legacy SDKv2 provider is renamed to `fastly/fastly-legacy` at GA. If the
  legacy provider is no longer published under `fastly/fastly`, the legacy phase
  must use the new source address.
