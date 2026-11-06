# `fastly-tf-migrate migrate --dry-run`

CDTOOL-1906 adds a **read-only**, conservative migration planner. It consumes
the existing analyzer and compatibility registry and uses the source-preserving
`rewrite.Build` API to produce real proposed changes. It never calls `Apply`,
Terraform, or the Fastly API.

## Current preview coverage

- Automatic registry resource rules with `HCLUnchanged` and `StateUnchanged`
  require no file edits. The complete set is exercised by tests derived from
  `compatibility.Default().All()`.
- `fastly_service_vcl` to `fastly_service_cdn_auto`, with a `moved` block in
  the same module as the resource. Preserves comments, expressions, `count`,
  provider selection, and existing compatible nested blocks.
- Registry-defined automatic attribute removal, safe block renames, and
  top-level CDN settings moved into a `settings {}` block. The writer refuses
  HCL layouts its source-range primitives cannot safely preserve.
- Repeated local modules and multiple `.tf` files, with deterministically
  ordered output and cross-file target address checks.

## Deliberate blockers

The planner prints **no partial diff** if a required conversion is missing.
This includes product enablement splits (including explicit `null`), direct
references to renamed resources, constrained Fastly provider requirements,
remote module contents, and all provider-blocked/unsupported registry rules.
Nested `authentication`, `tls`, and `ssl_client_secrets` targets are
`schema.SingleNestedAttribute` objects, not HCL blocks: they require a separate
source-preserving object-attribute writer. The registry's automatic
`rate_limiter_id` rename is also blocked because the target ID is computed-only.
These require the corresponding implementation tickets.

This command does not claim to validate provider state schema versions, run
`terraform plan`, or guarantee that the dual-model provider version deployed
in a workspace implements every required state move. The future write-mode
command must enforce those prerequisites as well.

Use:

```console
fastly-tf-migrate migrate --dry-run [PATH]
```

Without `--dry-run`, `migrate` returns usage error 2; write mode remains out
of scope. Status 3 means preflight or planning blockers, while status 1 is
reserved for execution/IO/output failures.

`testdata/vcl` intentionally uses actual unchanged Fastly service fields.
Schema-independent rewrite-primitives are tested in `internal/migration/rewrite`
and are not examples of supported real-world Fastly field migrations.

**Note:** The diff contains original configuration text and can include literal
credentials or other sensitive values already present in `.tf` files. Treat
preview output as sensitive and avoid publishing it in CI logs or tickets.
