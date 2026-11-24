# Traffic-block configuration validation

Run `make test-migration-traffic-config` to check the CDN traffic-block test
configurations in `internal/migration/migrate/testdata/traffic/` without a
Fastly API token. The script builds the local provider and migration CLI,
migrates each configuration, compares the result with the expected output,
and runs `terraform validate` against the local provider via `dev_overrides`.
The migration CLI requires a lockfile that selects a legacy release, so the
script writes a placeholder one for 10.3.0 and removes it before validating.

This validates HCL and target schemas only. The live state move, director
backend associations and no-op plan are covered by `make test-migration-traffic`
(see `scripts/test-migration-vcl/README.md`), which requires `FASTLY_API_TOKEN`.
