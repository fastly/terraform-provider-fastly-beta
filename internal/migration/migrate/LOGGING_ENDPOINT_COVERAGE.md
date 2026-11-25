# VCL logging migration coverage — published SDKv2 10.3.0

Source of truth: `internal/resources/servicecdnauto/testdata/legacy_service_vcl_schema_v1_10_3_0_type.json`.
The published VCL SDKv2 schema contains **29** logging block types.
The 28 target-supported types are divided among VCL-05, VCL-06 and VCL-07; Logentries has no target and is **explicitly unsupported**.

| Work package | SDKv2 endpoint | CDN migration status | Compute coordination |
| --- | --- | --- | --- |
| VCL-05 | `logging_https` | Existing VCL-05 coverage | Existing rules; verify separately |
| VCL-05 | `logging_elasticsearch` | Existing VCL-05 coverage | Existing rules; verify separately |
| VCL-05 | `logging_kafka` | Existing VCL-05 coverage | Existing rules; verify separately |
| VCL-05 | `logging_splunk` | Existing VCL-05 coverage | Existing rules; verify separately |
| VCL-05 | `logging_syslog` | Existing VCL-05 coverage | Existing rules; verify separately |
| VCL-06 | `logging_s3` | Existing VCL-06 coverage | Existing rules; verify separately |
| VCL-06 | `logging_gcs` | Existing VCL-06 coverage | Existing rules; verify separately |
| VCL-06 | `logging_bigquery` | Existing VCL-06 coverage | Existing rules; verify separately |
| VCL-06 | `logging_blobstorage` | Existing VCL-06 coverage | Existing rules; verify separately |
| VCL-06 | `logging_cloudfiles` | Existing VCL-06 coverage | Existing rules; verify separately |
| VCL-06 | `logging_digitalocean` | Existing VCL-06 coverage | Existing rules; verify separately |
| VCL-06 | `logging_openstack` | Existing VCL-06 coverage | Existing rules; verify separately |
| VCL-06 | `logging_googlepubsub` | Existing VCL-06 coverage | Existing rules; verify separately |
| VCL-06 | `logging_kinesis` | Existing VCL-06 coverage | Existing rules; verify separately |
| VCL-07 | `logging_datadog` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_ftp` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_sftp` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_loggly` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_grafanacloudlogs` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_scalyr` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_newrelic` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_newrelicotlp` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_heroku` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_honeycomb` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_logshuttle` | Automatic (VCL-07) | Blocked for nested authentication; CDTOOL-1867 |
| VCL-07 | `logging_papertrail` | Automatic (VCL-07) | Existing rules; verify separately |
| VCL-07 | `logging_sumologic` | Automatic (VCL-07) | Existing rules; verify separately |
| VCL-07 | `logging_fos` | Automatic (VCL-07) | Blocked; CDTOOL-1867 |
| Unsupported | `logging_logentries` | **Rejected; no target block** | **Rejected; no target block** |

## VCL-07 implementation policy

- Legacy flat authentication members become a single Framework `authentication` object only through explicit registry rules. FTP/SFTP use `user` plus their supported password/SSH-key members; nine token endpoints move `token`.
- Papertrail and Sumo Logic retain their compatible flat endpoint fields. Sumo Logic pins the SDKv2 `message_type = "classic"` default (Framework defaults to `"blank"`).
- FOS exists in the published **SDKv2 10.3.0** schema and already has a Framework target block and legacy state decoder. Only **CDN FOS** is enabled here. The Compute FOS rule remains blocked pending CDTOOL-1867.
- The migrator preserves configured `format` expressions. For omitted or explicit-null VCL log formats, it pins byte-exact SDKv2 defaults. FTP, Datadog, New Relic, New Relic OTLP and Honeycomb have distinct historical formats; the other VCL-07 endpoints reuse the SDKv2 shared JSON format.
- Invalid/ambiguous SFTP credentials, unknown flat attributes, invalid static compression, missing required fields, blank formats and unsupported placement are rejected **before writing**. Nonliteral expressions are kept for target validation.
- Migrator previews suppress all raw HCL diffs containing these endpoint types. Do not copy Terraform state, trace logs, or secrets into PR comments.

## Verification and remaining gaps

Offline suite: `go test ./internal/migration/...` and `make test-migration-logging-remaining-config`. These cover all fourteen endpoints, published historical schemas, negative paths, the 29-type inventory, golden HCL and Terraform target validation.

Opt-in API-backed scenarios: `make test-migration-logging-remaining-live` (Papertrail) and `make test-migration-fos-live` (FOS with test-owned bucket and entitlement). Both create and destroy a disposable Fastly CDN service and require `FASTLY_API_TOKEN`.

**No live tests were executed when preparing this patch.** Live endpoint creation/activation, FOS entitlement requirements, API-returned format defaults and any redacted credentials must be verified in an isolated Fastly test account. Do not claim every one of the fourteen endpoints has been live-tested. Logging delivery to the external receiver is not tested. A migration plan proposing remote service mutations is a blocker, not an acceptable fallback.

Shared Compute authentication and Compute FOS migration remain outside this work package; coordinate with **CDTOOL-1867**. Also confirm the VCL state-migration prerequisites in **VCL-01** before release-version claims.
