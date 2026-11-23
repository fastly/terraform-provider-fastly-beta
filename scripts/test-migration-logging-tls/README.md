# Logging TLS/authentication configuration validation

Run `make test-migration-logging-tls-config` to check the CDTOOL-1923
migrations for CDN and Compute without a Fastly API token or external logging
receiver.

The script builds the local Framework provider and migration CLI, migrates
the repository-owned synthetic combined logging fixtures, compares the written
HCL with the reviewed golden files, and runs `terraform validate` against
the locally built provider via `dev_overrides`.

A synthetic SDKv2 10.3.0 `.terraform.lock.hcl` is seeded solely so the CLI's
source-version safety gate can check a selected legacy release. The script
verifies that migration leaves it untouched, then removes it *only from the
disposable test directory* before validating the target HCL. It deliberately
does not run `terraform init`: there is no published GA 20.x provider to
resolve yet, and Terraform successfully validates with the development
override in a clean test workspace.

```bash
make test-migration-logging-tls-config
```

This validates HCL transformations and target provider schemas, **not** live
logging-endpoint state or remote credential preservation. All five endpoint
types have golden/schema coverage, but this validation suite does not exercise
receiver-backed migration. On a real migration, independently review
endpoint state and credentials and require a no-op Terraform plan before
applying a state move. Never use the synthetic fixtures against Fastly.
