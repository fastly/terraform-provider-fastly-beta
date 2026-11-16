# Configurable product migration end-to-end test

Exercises `fastly-tf-migrate` against live Fastly resources for Bot Management,
DDoS Protection and NGWAF with non-default settings (plus WebSockets).

## Flow

Run with `enabled`, `disabled` or `all` (default). Each scenario gets its own
service and its own throwaway NGWAF workspace (`mode: off`), created via the API
unless `FASTLY_TEST_NGWAF_WORKSPACE_ID` is set (never deleted in that case).

**`enabled`** (non-default settings: contentguard `on`, mode `block`,
traffic_ramp `25`, plus WebSockets):

1. Apply a legacy `fastly_service_vcl` with `product_enablement` using the
   published legacy `fastly/fastly` provider, then `apply -refresh-only`.
2. Snapshot the live status and configuration of each product through the API
   and assert contentguard, mode, workspace_id and traffic_ramp.
3. `fastly-tf-migrate analyze`, `migrate --dry-run`, `migrate --write`; assert the
   legacy block is gone and the settings appear in `migration.tf`.
4. Switch to the locally built provider (served via `dev_overrides` for
   `fastly/fastly`; `terraform init` is skipped because no 20.x release exists on
   the registry) and plan. Every action must be `no-op`/`read`: 4 imports, and no
   create, update, delete or replace.
5. Apply, then require a no-op `plan -detailed-exitcode`.
6. Re-snapshot the products; the snapshot must be identical, and the four
   settings must be unchanged.
7. Check the provider's HTTP calls during steps 4-5, taken from the go-fastly
   debug dump (`FASTLY_DEBUG_MODE`, API key header removed) in the Terraform
   trace: no non-GET request may reach `/enabled-products/` or `/ngwaf/`.
8. Destroy.

**`disabled`** (`enabled = false` on all three configurable products): the
migrator must refuse. Importing a disabled product plans an in-place update
(the target `Read` does not populate its settings) that calls the disable API, so
`migrate --dry-run` and `--write` must exit non-zero, name each product, and
leave every `.tf` file untouched. The legacy service is then destroyed.

On failure, an emergency destroy and a name-prefix API sweep run, and the
temporary workspaces are deleted.

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
