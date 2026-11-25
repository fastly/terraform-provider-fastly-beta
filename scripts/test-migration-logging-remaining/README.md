# CDTOOL-1925 VCL-07 test harness

**Offline configuration verification**

Run `make test-migration-logging-remaining-config` with Go 1.26.11 and
Terraform installed. This builds the local Framework provider and migration
CLI, runs the migrator on the independent fourteen-endpoint golden HCL and
verifies that the target HCL passes `terraform validate`. No Fastly service
or receiver is created.

**Opt-in Papertrail API migration**

Run `FASTLY_API_TOKEN=... make test-migration-logging-remaining-live` using an
isolated Fastly test account. The script creates and destroys a disposable
CDN service, configures a synthetic Papertrail address (logging delivery is
**not** tested), migrates SDKv2 10.3.0 state, and demands a zero-action move
plan, final no-op, unchanged Fastly ID/active version and endpoint state.
During migration it inspects provider HTTP traces and rejects service/product
mutation requests. It retains private logs on error for diagnosis.

**Opt-in FOS API migration**

FOS requires a real **test-owned** Fastly Object Storage bucket and the
appropriate entitlement. Do not use customer buckets. Run:

```bash
export FASTLY_API_TOKEN=...             # test account
export FASTLY_FOS_TEST_BUCKET=...       # test-owned bucket
export FASTLY_FOS_TEST_REGION=...       # matching Fastly FOS region
make test-migration-fos-live
```

The FOS scenario creates and removes the **logging endpoint and test CDN
service**, not the storage bucket. It requires exact SDKv2 10.3.0 by default.
Check manually that no objects were written to the test bucket and delete
any test data as appropriate. The script does **not** verify object delivery.

Both scripts require `go`, `terraform`, `jq`, `curl` and a working SDKv2
registry install. On a failed run they attempt cleanup of their own
uniquely named test service and retain private logs; inspect those logs
locally. Do **not** attach Terraform state, plans, traces or credential
values to Jira/GitHub.

Both live scenarios are opt-in and not run by routine CI. They are **not**
claimed as executed in the patch preparation environment. Authentication
and actual receiver delivery for the other twelve endpoints require
separate account-owned integrations and should not be claimed as tested.
