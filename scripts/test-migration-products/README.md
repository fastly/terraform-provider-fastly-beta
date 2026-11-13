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
