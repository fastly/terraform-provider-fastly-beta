# Legacy SDKv2 source-version compatibility (CDTOOL-1914)

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
| v10.1.0 (latest checked) | 1 | Same |

Terraform upgrades v0 state to v1 only when the 9.1.0+ provider reads it
(`plan`, `apply` or `apply -refresh-only`). Below 9.1.0 the persisted state is
v0, which the mover rejects, so **9.1.0 is a technically necessary floor** for
`fastly_service_vcl`. There is no ceiling in the tested range (through 10.1.0).

Evidence is the released provider source at each tag (`gh api` against
`fastly/terraform-provider-fastly`), not recorded state files. The boundary is
exercised by `analyze/source_version_test.go` (`8.8.0`, `9.0.0` blocked;
`9.1.0`, `9.8.0`, `10.1.0`, `19.9.9` accepted; `20.0.0`, `20.1.0` blocked).

`fastly_service_compute` is blocked separately in the registry and carries no
source-release floor here.

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
  target, not a legacy source. Target-provider requirements are CDTOOL-1865.
- Lockfile missing, unreadable, malformed, or without a usable legacy entry:
  blocking prerequisite finding (initialize with the legacy provider, keep the
  lockfile, do not use `terraform init -upgrade`).
- No legacy service resources (for example an already-migrated project): no check.

`analyze` still prints all normal findings; dry-run and write return exit code 3
with no diff and no writes. A `required_providers` range such as `~> 9.0` is
never treated as the selected version.

## Limits

The lockfile records the release Terraform selected at `init`. It does not prove
which release last wrote the state, so a user who upgraded the lockfile without
refreshing state still has v0 state. The persisted schema is checked by the
separate safety-gates ticket. Remote state retrieval and state parsing are out of
scope here.
