# Product enablement migration live acceptance tests

Exercises **every currently automatic CDN product migration** in
`fastly-tf-migrate`: seven simple on/off products plus the three configurable
products. This is deliberately separate from the product-free VCL state-move
suite and the Compute state-move suite.

## Coverage and selectors

| Command | Live scenario | Expected result |
| --- | --- | --- |
| `./scripts/test-migration-products/run.sh simple` | Each simple CDN product independently (`enabled = true`) | Seven successful one-product imports |
| `./scripts/test-migration-products/run.sh simple:domain_inspector` | One named simple product | One import and final no-op plan |
| `./scripts/test-migration-products/run.sh enabled` | Bot Management (`contentguard=on`), DDoS Protection (`mode=block`), NGWAF (`workspace_id`, `traffic_ramp=25`), plus WebSockets | Four imports and final no-op plan |
| `./scripts/test-migration-products/run.sh disabled` | All three configurable products with `enabled=false` | Migrator blocks before writing anything |
| `./scripts/test-migration-products/run.sh configurable` | Both existing configurable scenarios | Enabled adoption and disabled safe rejection |
| `./scripts/test-migration-products/run.sh all` | All simple products and both configurable scenarios | Full currently automatic CDN coverage |
| `./scripts/test-migration-products/run.sh list` | Print available products/selectors | No credentials or API access needed |

The **simple** CDN products are `api_discovery`, `brotli_compression`,
`domain_inspector`, `image_optimizer`, `log_explorer_insights`,
`origin_inspector`, and `websockets`. The configurable products are
`bot_management`, `ddos_protection`, and `ngwaf`.

For **every enabled product scenario**, the script:

1. Creates a fresh legacy `fastly_service_vcl` with the product enabled, then
   runs the legacy provider's `terraform apply -refresh-only`.
2. Snapshots the enabled-products API status (and configuration for configurable
   products).
3. Runs `fastly-tf-migrate analyze`, `migrate --dry-run`, `migrate --write`.
   Checks generated standalone resources/import blocks and the VCL `moved` block.
4. Switches to the locally built dual-model provider with a Terraform development
   override for `fastly/fastly`, verifies `previous_address` and exact import
   count, and refuses *any* plan with create/update/delete/replace actions.
5. Applies the import/move, verifies the Fastly service ID and new state
   addresses, requires `terraform plan -detailed-exitcode` to return 0, and
   checks that no mutating product API calls appeared in the provider trace.
6. Compares live API snapshots before/after and destroys the temporary service.

The existing `disabled` test asserts that all three configurable products
fail migration without modifying HCL, since importing them with `enabled=false`
would otherwise trigger a disable API call. It destroys its test service
using the legacy provider.

**Scope:** `fanout` is Compute-only and the Compute product compatibility
rules remain blocked in this repository version. Therefore this suite covers
all *automatic CDN product transformations*, not every Fastly product or
future Compute migration. Do not claim complete coverage if an individual
scenario has not actually passed.

## Prerequisites and execution

Requires `terraform` (1.5+), `go`, `jq`, `curl` and `FASTLY_API_TOKEN`.
Use a **dedicated non-production Fastly account** with the entitlements for
the products being tested. Each scenario creates and destroys a *real*
Fastly service. The configurable scenarios also create/delete an NGWAF
workspace unless `FASTLY_TEST_NGWAF_WORKSPACE_ID` is provided; a pre-existing
workspace is never deleted.

```bash
./scripts/test-migration-products/run.sh list
export FASTLY_API_TOKEN=... # never commit the token

# Run an individual product first (requires only its entitlement):
./scripts/test-migration-products/run.sh simple:domain_inspector

# Run all simple products, or the existing configurable scenarios:
./scripts/test-migration-products/run.sh simple
./scripts/test-migration-products/run.sh configurable

# Run everything (requires all applicable entitlements):
make test-migration-products
# Or choose a subset via Make:
make test-migration-products SCENARIO=simple:websockets
```

`all` is the default selector for both the script and Make target; a scenario
failure stops the test with a nonzero exit code. There is **no silent
skip** for products missing an entitlement, so a successful `all` run actually
covers every listed product. To isolate an entitlement or API issue, run the
named `simple:PRODUCT` scenario separately.

**Image Optimizer:** Fastly rejects enablement when any backend lacks a
shield. The `simple:image_optimizer` fixture therefore sets its backend
`shield = "amsterdam-nl"` (using the location in the repository's existing
shielded-backend fixture). Other product fixtures stay unshielded.

`LEGACY_PROVIDER_VERSION` sets the legacy minimum (default `9.5.0`);
`KEEP_WORK_DIR=1` preserves temporary files after successful runs.
On failure, the work directory is always retained for inspection. It may
contain credentials in Terraform state or traces: **keep it private** and
delete it securely after investigation. The test attempts service and
workspace cleanup, but verify orphaned services manually after failures.

## GA provider follow-up

The acceptance script currently tests the pre-GA dual-model provider from a
local `go build` with a Terraform `dev_overrides` entry for `fastly/fastly`.
The migrated `>= 20.0.0` provider requirement cannot yet be installed from
the registry in this workflow, so the script skips a second `terraform init`
and removes the stale legacy lock entry.

Once the GA provider is released, replace the local override with
`terraform init -upgrade` and test the published provider and its actual
source-address transition. The legacy test phase currently uses the published
SDKv2 `fastly/fastly` and `FASTLY_API_KEY`, while the local dual-model phase
uses `FASTLY_API_TOKEN`. This suite is opt-in because it requires
live credentials and product entitlements; it is **not** run in CI or
`release-check`.
