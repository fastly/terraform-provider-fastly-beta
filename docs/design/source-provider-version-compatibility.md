# Legacy SDKv2 source-version compatibility

## Conclusion: go, with a minimum source release of `fastly/fastly` 9.1.0

`fastly_service_cdn_auto` moves state only from `fastly_service_vcl` persisted
at **schema version 1** (`internal/resources/servicecdnauto/state_migration.go`).
That schema version is tied to the legacy provider release that last wrote the
state:

| Legacy release | `fastly_service_vcl` `SchemaVersion` | Evidence |
| --- | --- | --- |
| v9.0.0 and earlier (checked v9.0.0; v8.x and older predate any upgrader) | 0 (unset) | `fastly/resource_fastly_service_vcl.go` at the tag sets no `SchemaVersion` or `StateUpgraders` |
| v9.1.0 | 1 | Same file sets `resource.SchemaVersion = 1` with a v0 upgrader (bot_management boolean to list, PR #1226) |
| v9.8.0 | 1 | Same |
| v10.0.0 | 1 | Same |
| v10.1.0 | 1 | Same |
| v10.2.0 | 1 | Backend IAM attributes added for VCL |
| v10.3.0 | 1 | FOS logging block added |

Terraform upgrades v0 state to v1 only when the 9.1.0+ provider reads it
(`plan`, `apply` or `apply -refresh-only`). Below 9.1.0 the persisted state is
v0, which the mover rejects, so **9.1.0 is a technically necessary floor** for
`fastly_service_vcl`. A stable schema version does not imply a stable persisted value type. The
10.3.0 source still declares version 1, but compatibility across its additive
schema changes must be established by the regression suite below.

Evidence is the released provider source at each tag (`gh api` against
`fastly/terraform-provider-fastly`), not recorded state files. The boundary is
exercised by `analyze/source_version_test.go` (`8.8.0`, `9.0.0` blocked;
`9.1.0`, `9.8.0`, `10.1.0`, `19.9.9` accepted; `20.0.0`, `20.1.0` blocked).

`fastly_service_compute` is blocked separately in the registry and carries no
source-release floor here. Its provider-side mover accepts schema version 0;
the compatibility suite exercises candidate releases starting with 9.5.0,
the original Compute fixture release. Compute CLI support and its source
version policy remain outside this change.

## Additive schema drift

The original checked-in type snapshots are retained as historical fixtures.
The current decoder types also recognize the following changes identified
from the legacy source:

- 9.2.0: VCL backend `max_use` and `max_lifetime` (already in the original
  snapshot; the 9.1.0 test shape omits them).
- After 9.8.0, by 10.2.0: `logging_gcs.public_key` for VCL and Compute
  (absent in the published 9.8.0 schemas; exact introduction release not established).
- 10.2.0: VCL backend IAM fields.
- 10.3.0: `logging_fos` for VCL and Compute.

The new 10.3.0 fixture types are reconstructed from source and the existing
snapshots. `make test-migration-schemas` independently checks them and the
historical shapes against exact published registry binaries, and exercises
the provider protocol MoveResourceState RPC. See
[schema compatibility tests](../../scripts/test-migration-state-schemas/README.md)
for the live state-move matrix. Both the published-schema check and live
matrix must pass before treating these releases as verified.

Configured backend IAM remains blocked in both CLI analysis and the VCL
state mover until target parity is implemented. Configured FOS remains
blocked in CLI analysis until its configuration migration is verified.
Recognizing a field in a decoder is not a claim of configuration migration
support.

## What the check does

One read-only check runs inside `analyze.Analyzer.Analyze`, which `analyze`,
`migrate --dry-run` and `migrate --write` all share, when a rule with a
`MinSourceProviderVersion` has an action in the tree. It reads only the
`registry.terraform.io/fastly/fastly` entry of `.terraform.lock.hcl` in the
selected root.

- Lockfile version `>= 9.1.0, < 20.0.0` (`MaxSourceProviderVersion` is exclusive;
  20.0.0 is the first dual-model GA release): no finding.
- Lockfile version `< 9.1.0`: blocking finding with upgrade remediation. If a
  `required_providers` constraint excludes 9.1.0 the remediation says to relax it.
- Lockfile version `>= 20.0.0`: blocking finding; that release is the migration
  target, not a legacy source. The migration CLI rewrites the target provider
  requirement only after source preflight succeeds.
- Lockfile missing, unreadable, malformed, or without a usable legacy entry:
  blocking prerequisite finding (select a legacy release within `>= 9.1.0, < 20.0.0`
  with `terraform init`, or `terraform init -upgrade` run by the user; the CLI
  never upgrades the provider; keep the lockfile).
- No legacy service resources (for example an already-migrated project): no check.

`analyze` still prints all normal findings; dry-run and write return exit code 3
with no diff and no writes. A `required_providers` range such as `~> 9.0` is
never treated as the selected version.

## Limits

The lockfile records the release Terraform selected at `init`. It does not prove
which release last wrote the state, so a user who upgraded the lockfile without
refreshing state can still leave v0 state. This lockfile check does not verify
the persisted state schema; that requires a separate safety check. Remote state
retrieval and state parsing are out of scope here.

The preflight range is a release-family policy, not a claim that every version
in that range has been tested. Releases beyond the matrix require separate
schema and live-state validation.
