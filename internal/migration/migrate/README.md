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
- Compatible existing nested blocks, expressions, comments, `count`,
  `for_each`, provider meta-arguments and unrelated HCL remain unchanged.
- Registry-defined automatic removals, safe block renames and settings
  nesting are compiled using the existing rewrite primitives.
- Local child modules and multiple `.tf` files are planned deterministically.
- Legacy `fastly/fastly` requirements are updated to `>= 20.0.0` without
  changing their provider source address. Missing requirements are added.
- Supported provider arguments are renamed without changing values, aliases,
  provider references, or unrelated providers.

Any **unsupported** transformation blocks the *entire* plan without writing.
This includes product enablement splits (even explicit `null`), direct
references requiring rewrite, ambiguous/exclusionary provider constraints,
unsupported provider arguments, remote module internals, and the current unsupported
nested object-attribute targets (`authentication`, `tls`,
`ssl_client_secrets`) and computed-only rate-limiter ID. These remain for
the corresponding dedicated migration tickets. The write command does
**not** claim all legacy Fastly configurations are supported.

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

## Write safety and backups

When `--write` is requested:

1. Run the same analysis and completeness checks as `--dry-run`.
2. Print the **actual deterministic diff**. Any preview output error stops
   the command before creating backups or touching Terraform files.
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

**Security:** Dry-run diffs and backups can contain literal credentials from
`.tf` files. Do not post their contents to shared tickets or CI logs. Backups
are written privately outside the repository; they remain until manually
removed.
