# Storage/cloud logging configuration validation

Run `make test-migration-logging-storage-config` from the repository root.
This builds the local provider and migration CLI, uses synthetic SDKv2
10.3.0 lockfile fixtures, migrates both CDN and Compute HCL, compares the
results to independently authored golden output, and invokes `terraform
validate` with local `dev_overrides` (no API token, registry download,
Terraform state, or live Fastly service).

A successful configuration test is **not** proof that the Fastly API returns
unredacted credential values, preserves an endpoint's active service version,
or produces a no-op plan after a real provider upgrade.

## Opt-in S3/CDN and S3/Compute Fastly API migration acceptance tests

For each service family, the acceptance harness creates a disposable
Fastly service with S3 logging configured using **public dummy values**
from the `go-fastly` test recordings:

- `go-fastly/fastly/fixtures/s3s/create.yaml`
- `go-fastly/fastly/logging_s3_test.go`

The checked-in values fixture is `live-fixtures/s3-cdn/test-values.sh`,
shared by the CDN and Compute test configurations. It uses the
documented AWS example access key and secret and bucket `bucket-name`.
**These are not functional AWS credentials or a real test-owned bucket.**
Fastly's logging configuration API accepts these fields as endpoint
configuration in the recorded requests; whether the currently deployed Fastly
API accepts the full service creation/activation is verified by the live run.
No S3 delivery or receiver ownership is asserted.

Run the API-backed acceptance test, with **only one required environment
variable**:

```bash
# Use an isolated Fastly test account. The token must be authorized to
# create and delete disposable Fastly services.
export FASTLY_API_TOKEN=...  # supply via secret manager
make test-migration-logging-storage-live
```

`make test-migration-logging-storage-live` itself is the explicit opt-in; do
not run it on a production account or in routine CI. It creates and destroys
**two real disposable Fastly services** (CDN followed by Compute),
pins SDKv2 10.3.0 by default, and
builds the local target provider. There are no required `LOGGING_STORAGE_*`
environment variables, AWS credentials, or S3 buckets. The Compute case
reuses `internal/acceptance_tests/fixtures/packages/valid.tar.gz` and
Fastly's `fastly_package_hash` data source from the existing Compute
migration tests. The Compute fixture deliberately omits VCL-only S3
`format` and `format_version` attributes, which the SDKv2 Compute schema
does not support. Fastly-side logging entitlements/permissions are still
account-dependent.

All Terraform state, private logs, plan files and API traces stay in a
private (`0700`) temporary directory, removed after successful cleanup.
On failure the directory is retained for investigation; it may contain
`FASTLY_API_TOKEN` in HTTP traces. Never upload these files to GitHub.
If Terraform teardown cannot confirm removal, the harness uses the
repository's `cleanup-test-services` utility scoped to that run's unique
test-service name and verifies the result. Failed runs also sweep for orphaned
services after partial applies. If API cleanup fails, delete the temporary
service manually.

**Automated gates**:

1. For **each** service family, create a fresh SDKv2 service with a
   credentialed S3 endpoint and verify a
   legacy no-op plan, non-redacted credentials in state, and live service ID.
2. Check migration read-only preview does not write files or disclose credentials;
   verify the CLI produces the nested authentication object and a moved block.
3. `terraform validate`, then inspect the saved migration plan and **refuse to
   apply** if any managed resource would be created, updated, imported, replaced
   or destroyed.
4. Apply only the state move, require final no-op, compare all common S3 endpoint
   state fields and exact credentials against the legacy snapshot, and compare
   live service ID/name/active version.
5. Audit the provider trace for service reads and reject mutating service and
   product API calls during migration (the final cleanup destroy is excluded).
6. Destroy each isolated test service in an EXIT trap. On failed runs,
   use the existing exact-run-name API cleanup utility to recover services
   that may not have been captured in Terraform state; report a failure
   if their deletion cannot be verified.

The test does not send traffic, authenticate with AWS, or check S3 object
delivery. Because the credentials are deliberately fake, **this is API-backed
logging endpoint configuration/state acceptance, not receiver-backed
delivery acceptance**. A failure due to credential redaction or provider
refresh is a **real migration safety blocker**, not a reason to weaken the
test. The script runs only when explicitly invoked.

### Acceptance evidence for the other eight endpoints

The nine-endpoint CDN/Compute golden HCL and schema tests run without
external receivers. The opt-in S3 API-backed fixture provides a reproducible
representative end-to-end Fastly service/state migration without a real S3
receiver. It does **not** prove delivery to S3, or live credential
preservation for GCS, BigQuery, Blob Storage, Cloud Files, DigitalOcean,
OpenStack, Google Pub/Sub or Kinesis. CDN and Compute S3 use the same
Fastly API-backed acceptance checks. Record each family's run result
independently.

For each additional receiver-backed endpoint, provision a test-owned bucket,
dataset, container, stream or subscription with disposable credentials; use
a separate test Fastly service. Establish a no-op plan on legacy SDKv2 10.3.0,
record the service ID, active version and exact logging configuration privately,
then run migration dry-run/write and move to the Framework provider. Verify
the endpoint configuration and *unredacted* credential state, unchanged
service ID/active version, a migration plan with zero managed-resource actions,
no mutating service/product API requests, and a final Terraform no-op plan.

If any API response masks a secret or the target provider cannot reconstruct
it, **block migration** and do not apply a plan that changes the service or
logging configuration. Record which endpoint and SDKv2 release were exercised,
entitlement and fixture limitations, and any detected redaction. Test runs
must be reported in the PR; a checked-in harness is not a recorded pass.

### Running and reporting results

`make test-migration-logging-storage-live` executes **CDN first, Compute
second**. It fails immediately if either plan contains managed-resource
actions and retains the private test directory. Keep the temporary state,
traces, and Terraform plan JSON out of PR attachments because they may contain
sensitive credentials or API tokens.

PR evidence should list the exact SDKv2 version and independently record
pass/fail results for CDN S3 and Compute S3. Do not substitute an earlier
run or one family's result for the other; only report executed acceptance
tests as passes.

The other eight endpoint types have offline golden HCL/schema checks, not
API-backed migration verification. Public `go-fastly` recordings alone do
not establish valid destination infrastructure or credential preservation for
those services; report this scope limitation rather than silently treating
the S3 results as exhaustive logging acceptance.
