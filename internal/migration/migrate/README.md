# `fastly-tf-migrate migrate` — preview and explicit write mode

The write mode builds on the read-only migration planner. **Both commands
use the same compatibility-checked, immutable HCL rewrite plan.**

```console
fastly-tf-migrate migrate --dry-run [PATH]
fastly-tf-migrate migrate --write [--backup-dir PATH] [PATH]
```

`PATH` defaults to the current directory. `--dry-run` and `--write` are
mutually exclusive. Without either flag, the command returns a usage error.
Dry-run **never writes files** and returns exit code 0 for a complete,
supported preview whether or not it proposes changes. The initial GA CLI
intentionally does not include a CI-specific `--check` mode.

## Migration scope

The write command supports **exactly** the migrations for which the read-only
planner can produce a complete diff:

- Automatic registry rules with `HCLUnchanged` / `StateUnchanged` are no-ops.
- `fastly_service_vcl` becomes `fastly_service_cdn_auto`, with a Terraform
  `moved` block in the same module.
- `fastly_service_compute` becomes `fastly_service_compute_auto`, with a
  Terraform `moved` block in the same module. The provider's
  `MoveState` copies the service ID, `force_destroy` and `reuse`; `activate = false`,
  `stage = true` and other constructs the `_auto` schema cannot represent block
  the whole plan before anything is written.
- Compatible existing nested blocks, comments, `count`, `for_each`, provider
  meta-arguments and unrelated HCL remain unchanged. Supported references in
  their expressions are rewritten using parsed HCL syntax.
- Registry-defined automatic removals, safe block renames and settings
  nesting are compiled using the existing rewrite primitives.
- Local child modules and multiple `.tf` files are planned deterministically.
- Legacy `fastly/fastly` requirements are updated to `>= 20.0.0` without
  changing their provider source address. Missing requirements are added.
- Supported provider arguments are renamed without changing values, aliases,
  provider references, or unrelated providers.
- Literal simple CDN product enablements are split into standalone
  `fastly_service_product_*` resources and Terraform import blocks, using
  the moved CDN Auto service ID. Explicit `false` remains managed as disabled;
  `null`, empty, and omitted settings do not create product resources.

Any **unsupported** transformation blocks the *entire* plan without writing.
This includes unknown-expression product enablements,
unverified reference transformations, ambiguous/exclusionary provider constraints,
unsupported provider arguments, remote module internals, and the current unsupported
nested object-attribute targets (`authentication`, `tls`)
and computed-only rate-limiter ID. These remain for
the corresponding dedicated migration tickets. The write command does
**not** claim all legacy Fastly configurations are supported.

## Standalone domain migration

Versionless domain resources migrate together with the services that reference them:

| Legacy resource                 | Target resource              | HCL                         | State                                  |
| ------------------------------- | ---------------------------- | --------------------------- | -------------------------------------- |
| `fastly_domain`                 | `fastly_domain`              | Address kept, no `moved`    | Provider state upgrader (v0 to v1)     |
| `fastly_domain_v1`              | `fastly_domain`              | Renamed, `moved` block      | Provider `MoveState`                   |
| `fastly_domain_v1_service_link` | `fastly_domain_service_link` | Renamed, `moved` block      | Provider `MoveState`                   |
| `fastly_domain_service_link`    | `fastly_domain_service_link` | Unchanged                   | Unchanged                              |

The legacy `domain_id` attribute no longer exists; references to
`fastly_domain.<name>.domain_id` and `fastly_domain_v1.<name>.domain_id`
become `.id`. The provider drops the legacy `domain_id` value and turns an unset `description`
(persisted by SDKv2 as `""`) into null, so no domain is recreated and an omitted
description plans no change. An explicit `description = ""` takes one
empty-description update after migration.
The `id` and `fqdn` outputs are unchanged. An unset `description` or `service_id` reads as
null instead of `""` after migration, so an expression that compares them to `""` must be
checked by hand. Whole-resource
references, other attributes, projections that read `domain_id`, and `moved`, `import` or
`removed` blocks that address a migrating domain block the whole plan before anything
is written.

A module that declares a `fastly_domain_v1` or `fastly_domain_v1_service_link` resource moves to the
target provider requirement even when it has no service, because the legacy provider cannot run the
`moved` block. An unchanged `fastly_domain` updates an existing Fastly provider requirement or block
but never adds one, since the type is shared with the target provider.

**Verification status.** The SDKv2 domain schemas are verified against published
releases 9.1.0 through 10.3.0 by `make test-migration-schemas`. `make test-migration-domains`
is a live test that creates real services and domains in a test account that can create
versionless domains. It migrated state from SDKv2 10.3.0 with a zero-change plan and
no mutating domain or service requests, and left the services' ids and active versions
unchanged. It covers a direct association, omitted and populated descriptions, a
standalone link and both `_v1` renames. A linked domain keeps no description there,
because creating the link leaves the remote description empty.

The deprecated `fastly_domains_v1` data source is not migrated. The target
`fastly_domains` schema and computed `id` differ, so the CLI blocks the whole plan
with a finding instead of renaming it. Replace the data source and its references by
hand, then run the migration again.

## Sensitive nested object transformations

Legacy static CDN and Compute backend `ssl_client_cert` and `ssl_client_key`
attributes are moved into the target `ssl_client_secrets = { ... }` object.
The writer uses the exact original expression bytes and does not evaluate
them, read referenced files, or resolve variables. Repeated backends are
handled independently, preserving absent fields, explicit `null`, comments,
and supported references. Heredoc contents are copied without reindentation.
An existing target object mixed with legacy attributes, inline layouts
that cannot be separated safely, and dynamic service blocks are rejected.

For example:

```hcl
# Before
backend {
  name = "origin"
  address = var.origin
  ssl_client_cert = file(var.cert_file)
  ssl_client_key = local.client_key
}

# After
backend {
  name = "origin"
  address = var.origin
  ssl_client_secrets = {
    ssl_client_cert = file(var.cert_file)
    ssl_client_key = local.client_key
  }
}
```

The reusable `rewrite.NestObject` edit groups flat attributes by destination
and supports renamed object keys. Additional logging mappings must be
reviewed and enabled in the dedicated logging tickets. Standalone
`fastly_tls_private_key` and `fastly_integration` remain explicitly blocked
until their same-name state upgrade prerequisites and precise source schema
mappings are verified. The target TLS private key uses a required
`private_key.pem` object member and the target integration uses a sensitive
`authentication` string map. Both have values that cannot be recovered by
a remote read; rewriting HCL alone is insufficient. NGWAF alert integration
credentials are migrated separately; see the NGWAF section below.

## NGWAF resource migration

Renamed and split legacy NGWAF resources move to their dedicated resource
types, each with a module-local `moved` block. The provider moves the state
without changing the remote object; the first refresh rebuilds it from the API.

| Legacy | Target |
| --- | --- |
| `fastly_ngwaf_account_list`, `fastly_ngwaf_workspace_list` | `fastly_ngwaf_[workspace_]<type>_list`, selected by `type` |
| `fastly_ngwaf_account_rule` | `fastly_ngwaf_request_rule` or `fastly_ngwaf_signal_rule`, selected by `type` |
| `fastly_ngwaf_workspace_rule` | `fastly_ngwaf_workspace_{request,signal,rate_limit,templated_signal}_rule`, selected by `type` |
| `fastly_ngwaf_account_signal` | `fastly_ngwaf_signal` |
| `fastly_ngwaf_redaction` | `fastly_ngwaf_workspace_redaction` |
| `fastly_ngwaf_thresholds` | `fastly_ngwaf_workspace_threshold` |
| `fastly_ngwaf_virtual_patches` | `fastly_ngwaf_workspace_virtual_patch` (`action` becomes `mode`) |
| `fastly_ngwaf_alert_<kind>_integration` | `fastly_ngwaf_workspace_alert_<kind>_integration` |

The writer removes what the target type no longer has: the `type`
discriminator; `request_logging` on signal, rate limit and templated signal
rules; `action.type` on signal and templated signal rules; and the required
empty `description` on templated signal rules. Alert credentials (`key`,
`webhook`, or mailing-list `address`) move into `authentication = { ... }`,
keeping the original expressions. `fastly_ngwaf_workspace` and
`fastly_ngwaf_workspace_signal` keep their names and are not changed.

References are rewritten to the selected target, including `.action` to
`.mode` and alert credentials to `.authentication.<name>`. The following block
the migration before any write:

- a `type` that is not a literal, so the target cannot be selected statically;
- a signal rule `action.type` other than `exclude_signal`, a templated signal
  rule `action.type` other than `templated_signal`, or a templated signal
  `description` other than `""`, since the target cannot represent them;
- `dynamic "action"` blocks on signal or templated signal rules;
- references to outputs a target drops, such as `type` or account-list
  `applies_to`, and indexed `applies_to`, which is now a set.

The target schemas validate some values more strictly than the legacy
provider, such as list name lengths, signal name characters and condition
fields. The migrator does not repeat those checks: `terraform validate` and
`terraform plan` report them before any remote change.

`make test-migration-ngwaf` is an opt-in live test that creates one object for
each of the 28 targets with an exact legacy release, migrates them with the
CLI, and requires state-only moves, no NGWAF write requests, unchanged IDs and
a final no-op plan. It needs an account entitled to Next-Gen WAF and creates
account-wide lists, rules and a signal.

The CLI does not resolve or inspect remote state. Backend TLS migration
must be checked with Terraform's migration plan: the existing state movers
retain service identity and `Read` reconstructs backend fields from Fastly.
Never apply a migration-only plan that proposes backend changes. The
opt-in `make test-migration-backend-tls` exercise uses freshly generated
test certificates and keys and checks exact legacy and target TLS state,
stable service/consumer identities, no-op plans and no mutating API calls.

## Terraform reference rewriting

References are resolved against **enabled resource conversions in the same
source module**, never by textual search and replacement. Local module call
labels, module instance selectors, resource labels, provider aliases and
`providers` mappings are retained. A resource-level `moved` block in the
declaring module covers its `count` or `for_each` instances.

For example:

```hcl
# Before
output "service_ids" {
  value = fastly_service_vcl.demo[*].id
}
# After
output "service_ids" {
  value = fastly_service_cdn_auto.demo[*].id
}
```

The writer handles direct verified scalar outputs (including `id` and
`active_version`), static and dynamic instance indexes, splats, interpolation,
function arguments, conditional expressions, direct scalar instance
comprehensions, and dependency references. It also handles `self` and verified
relative `ignore_changes` paths. It preserves literal strings, comments and
lexically scoped `for` and `dynamic` iterators. It does not rewrite module
output names when the output declaration itself is migrated.

Legacy nested ACL, dictionary, dynamic snippet and rate-limiter collections
can be consumed by order-independent `{for item in collection : item.name =>
item.<identity>}` maps when the corresponding block rule is automatic. This
includes the `dynamicsnippet` block rename and the read-only rate-limiter ID
rename where the registry authorizes it. Other collection projections, numeric
positions into former sets, whole resource values and arbitrary nested output
paths require explicit semantic evidence.

The following are deliberately blocked before any write:

- Removed lifecycle outputs without a verified equivalent.
- CDN `settings` output references (including `default_ttl` and `http3`).
  Although the registry describes a singleton-list path, the migrated settings
  list is not yet guaranteed to be populated by the provider state mover.
  This applies even if the legacy setting is explicitly configured.
- Conflicting or indexed pre-existing moves, imports/removals targeting a
  migrated address, and possibly conflicting module-qualified state addresses.
  An exact existing module-local whole-resource move is preserved.
- Out-of-root or symlinked module sources, override files, JSON configuration,
  and unverified remote module contents.

Known mappings do not enable a blocked resource. Standalone domain upgrades
and legacy ACL-entry/dictionary-item resource migrations retain their existing
provider prerequisites. Their supported service-reference expressions use the
same writer when those resource conversions are enabled by their own tickets.

Both analysis and planning inspect references, and the final rendered HCL is
checked again before a write is offered. Newly added Terraform files invalidate
a prepared write. Run the CLI against a stable working tree and inspect
Terraform's actual plan before applying.

`make test-migration-references` is an opt-in live regression scenario with a
local module instance, aliased provider, counted VCL service, keyed Compute
service, cross-file references and a Terraform-managed consumer. It pins the
legacy provider using `LEGACY_PROVIDER_VERSION` (default `10.3.0`), checks exact
instance moves, no managed-resource changes, unchanged outputs and final no-op,
and destroys the two test services. Use an isolated Fastly test account.

## Legacy provider prerequisites

Before migrating `fastly_service_vcl`, prepare the project as follows. The same
check runs for `analyze`, `migrate --dry-run` and `migrate --write`.

1. Use legacy `fastly/fastly` **`>= 9.1.0, < 20.0.0`**. 9.1.0 is the first
   release that writes the supported `fastly_service_vcl` state schema 1;
   20.0.0 is the first release of the dual-model provider, which is the
   migration target, not a source.
2. Run `terraform init` with the legacy provider and keep the resulting
   `.terraform.lock.hcl` in the project root. If your lockfile selects a version
   below 9.1.0, constrain the provider to `>= 9.1.0, < 20.0.0` in
   `required_providers` and run `terraform init -upgrade` yourself to select a
   supported release. The migration CLI never runs Terraform or upgrades the
   provider for you. Do not rely on a loose range such as `~> 9.0`, which can
   allow older releases.
3. Run `terraform apply -refresh-only` with that provider so the state is
   persisted at schema 1.
4. Run `fastly-tf-migrate analyze`, review `migrate --dry-run`, then
   `migrate --write`. The CLI rewrites supported requirements to
   `fastly/fastly >= 20.0.0` together with the service configuration.
5. After reviewing the output, run `terraform init -upgrade` yourself to select
   the GA provider, then review `terraform plan` before any apply.

The lockfile records the selected release only. It is **not** evidence of which
provider version last wrote the state. If the lockfile is missing, malformed, or
selects a release outside that range, the check blocks with exit code 3 and no diff or
files are written. See `docs/design/source-provider-version-compatibility.md`.

## GA provider requirement and configuration migration

The first production dual-model release is **`fastly/fastly` v20.0.0**;
the old SDKv2 implementation is renamed to `fastly/fastly-legacy`.
The CLI preserves existing `source = "fastly/fastly"` addresses and
replaces supported legacy version constraints such as `~> 9.0` with
`>= 20.0.0`. Requirements in local child modules are updated separately.
Missing `required_providers.fastly` declarations are inserted in the
corresponding module (or its generated `migration.tf`).

For each `provider "fastly"` block, including aliases, the CLI renames:
`api_key` to `api_token`, and **literal absolute HTTPS**
`base_url` to `api_endpoint`. Value expressions and comments are preserved.
A dynamic or HTTP `base_url`, conflicting old/new attributes, unknown
arguments, and legacy-only `force_http2`/`no_auth` arguments block the
entire preview/write. Explicit provider-version exclusions and already
GA-only constraints with user restrictions require manual review rather
than silently discarding those restrictions.

**Environment variables:** SDKv2 `FASTLY_API_KEY` is **not** consumed by
the new provider: move credentials to `FASTLY_API_TOKEN` yourself.
`FASTLY_API_URL` is unchanged. The CLI cannot update shell/CI secrets.
`api_token` expressions are not interpreted. Review aliases and module
provider mappings in the generated diff.

No Terraform commands are run by the CLI. It does not modify
`.terraform.lock.hcl` or Terraform state. Keep a recovery copy of state,
review the diff, install the GA provider with `terraform init -upgrade`,
and verify the resulting `terraform plan` before applying changes.

## Simple product enablement migration

For legacy VCL services, the migrator splits statically known
`product_enablement` booleans into standalone versionless resources.
Supported products are Brotli Compression, Image Optimizer, Origin Inspector,
Domain Inspector, WebSockets, Log Explorer & Insights, and API Discovery.
Configurable products are split too: Bot Management (`enabled`, `contentguard`),
DDoS Protection (`enabled`, `mode`) and NGWAF (`enabled`, `workspace_id`,
`traffic_ramp`) must use literal values, which are copied into the generated
resource and adopted by import. Compute service migration remains blocked until
its separate prerequisites are met.

For example:

```hcl
# Legacy VCL service contains product_enablement { domain_inspector = true }
# After migration, the service is fastly_service_cdn_auto.demo, and
# migration.tf contains:
resource "fastly_service_product_domain_inspector" "demo" {
  service_id = fastly_service_cdn_auto.demo.id
  enabled    = true
}

import {
  to = fastly_service_product_domain_inspector.demo
  id = fastly_service_cdn_auto.demo.id
}
```

The import adopts the existing product by **Fastly service ID**, rather than
creating or enabling it. The moved service and import remain in the same
Terraform module, and an explicit service provider alias is copied to the
product resource. This requires Terraform with configuration-driven
`import` block support (Terraform **1.5+**).

`false` on a simple product generates a managed standalone product with
`enabled = false`. `enabled = false` on Bot Management, DDoS Protection or NGWAF
blocks the migration: importing a disabled configurable product plans an
in-place update (its settings are not read back) that calls the disable API.
Remove the block to leave the product unmanaged, or migrate it manually.
`null`, empty aggregate blocks, and omitted product fields generate no
product resource. Unknown boolean expressions, indexed services (`count` or
`for_each`), existing resource/import collisions, comments that would be
discarded from the legacy aggregate, and unsupported products or non-literal
product settings block migration without a partial diff or write.

**Critical:** The CLI does not read the actual product status or import the
product into Terraform state. After GA provider v20.0.0 is available and the
legacy state-schema prerequisite is satisfied, review the generated HCL,
run `terraform init -upgrade` yourself and inspect `terraform plan`.
The import expression must evaluate to the already-existing service ID at plan
time. Do **not** apply if Terraform proposes to create, re-enable, disable,
delete or replace the service or product unexpectedly. Live adoption and no-op
plan behavior require end-to-end acceptance testing; offline rewrite tests alone
cannot prove them.

## Write safety and backups

When `--write` is requested:

1. Run the same analysis and completeness checks as `--dry-run`.
2. Print the deterministic preview. Recognized sensitive configurations use
   a source-free preview with resource moves, files and compatibility actions
   instead of a raw HCL diff. Any preview output error stops the command
   before creating backups or touching Terraform files.
3. Acquire a per-project exclusive lock and reject unfinished backups from
   earlier interrupted runs.
4. Verify that all inspected `.tf` files still match the analyzer's snapshot.
5. Save every original file (mode `0600`) and a JSON manifest in a private,
   persistent backup directory **outside the Terraform root**. The manifest
   also identifies newly created files, original modes and SHA-256 checksums.
6. Apply the *same* immutable rewrite plan, with per-file staging, snapshot
   validation and best-effort rollback provided by the rewrite engine.
7. Mark the recovery manifest as `applied`, or `rolled-back` /
   `recovery-required` if a write fails.

The default backup root is `~/.fastly-tf-migrate`. To select another location,
pass `--backup-dir /private/location` (must be outside the Terraform tree and
not accessible to group/other users on Unix). Backups are kept until the user
deletes them; they can contain sensitive Terraform configuration.

The manifest is stored at
`<backup-root>/backups/<sha256-of-canonical-project-root>/backup-*/manifest.json`.
It records the root path, status, original/proposed hashes and paths. Copies of
existing originals live under `originals/<module-relative-path>`.
For files that were **created**, the manifest sets `"existed": false`.

### Recovery after interruption

**No multi-file HCL rewrite can be completely atomic on an ordinary
filesystem.** A process kill, crash, or power failure can leave some files
replaced and others unchanged. On an ordinary failure, the rewrite engine
attempts to roll back; persistent backups are the recovery fallback.
The write mode does **not** guarantee all-or-nothing replacement during a hard
crash, and does not automatically restore files that might have been edited
by another process.

If interrupted, stop other Terraform tooling, inspect the recorded
`manifest.json`, restore existing files from `originals/` (and their original
permissions as recorded in `mode`), and remove *only* newly created files
identified by the manifest **after checking their SHA-256**. Compare the full
Terraform directory to the pre-migration state. Only after reconciling the
files and confirming no migrator is still running should you resolve the
unfinished manifest/lock and re-run preflight. Never blindly remove a lock
or overwrite changed files. Keep the backup until a reviewed `terraform plan`
confirms the desired outcome.

`--write` does **not** run `terraform init`, `terraform plan`, `terraform
apply`, Fastly API calls, or direct state mutation. Terraform itself handles
provider-supported state moves when the user later runs `terraform apply`.
Always review provider version, state migration prerequisites and the final
Terraform plan before applying.

## Exit codes

| Code | Meaning |
| ---- | ------- |
| `0` | Completed preview or write, including no-op |
| `1` | I/O, backup, write, or internal failure |
| `2` | Invalid arguments / flag combination |
| `3` | Preflight blockers or incomplete transformation coverage |

The golden migration fixtures use actual supported Fastly schema fields;
generic HCL editing primitives are tested separately in
`internal/migration/rewrite`.

**Security:** For recognized sensitive fields anywhere in the analyzed module
tree, both dry-run and write previews withhold all source diffs, including
unchanged context and other files where values could be repeated. The preview
still reports file paths, moves and compatibility actions. This is a
conservative schema/name-based policy, not an arbitrary-secret detector:
ordinary non-sensitive previews can still contain user-supplied literals in
unrecognized fields. Do not publish raw HCL, state, plans or recovery backups.
Backups contain original unredacted HCL, are private outside the repository
and remain until manually removed. Existing source file permissions are
preserved.
