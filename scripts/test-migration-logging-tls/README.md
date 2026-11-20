# Logging TLS/authentication migration acceptance

`make test-migration-logging-tls-config` builds the local provider and
migrator, rewrites synthetic CDN and Compute fixtures and validates the
result using a development override. It requires Go and Terraform, but no
Fastly credentials or receiver. Do not apply the synthetic fixture.

`make test-migration-logging-tls` is opt-in and creates/destroys one real
VCL service in an isolated Fastly test account. It uses the exact published
SDKv2 release selected by `LEGACY_PROVIDER_VERSION` (default 10.3.0).
It intentionally does not create or provision logging receivers.

## Setup

Supply an absolute path to a trusted file containing *only* legacy logging
blocks to insert into the isolated service. Use receivers you control and
disposable credentials. Do not use customer endpoints, production credentials,
or untrusted HCL; Terraform file expressions and provider requests have
real side effects. The fixture directory and all secret files should be
private (`umask 077`, files mode 0600). The existing test service includes a
response condition named `migration-logging-response`. Use either that
condition or `placement = "none"` as appropriate for your receiver, not both.
No test traffic is sent intentionally. Verify receiver ownership/challenges,
TLS trust and forwarding prerequisites independently.

Example template — replace placeholder file paths with private test-owned
values before running. Remove optional credential fields you do not need.
The URL and TLS hostname must identify your actual receiver; certificate
and key must be valid for it. Use `trimspace(file(...))` where the SDKv2
validation rejects trailing whitespace.

```hcl
logging_https {
  name = "migration-https"
  url = trimspace(file("/absolute/private/https-url"))
  header_name = "Authorization"
  header_value = trimspace(file("/absolute/private/https-header"))
  tls_ca_cert = trimspace(file("/absolute/private/ca.pem"))
  tls_client_cert = trimspace(file("/absolute/private/client.crt"))
  tls_client_key = trimspace(file("/absolute/private/client.key"))
  tls_hostname = trimspace(file("/absolute/private/hostname"))
  gzip_level = 0
  placement = "none"
}
```

A Syslog TLS receiver is another representative choice; supply the actual
host, port, CA and token required by that receiver.

```hcl
logging_syslog {
  name = "migration-syslog"
  address = trimspace(file("/absolute/private/syslog-host"))
  port = 6514
  use_tls = true
  tls_hostname = trimspace(file("/absolute/private/syslog-host"))
  tls_ca_cert = trimspace(file("/absolute/private/syslog-ca.pem"))
  token = trimspace(file("/absolute/private/syslog-token"))
  placement = "none"
}
```

For Elasticsearch supply an HTTPS URL, index and optional user/password;
for Kafka supply reachable brokers, topic, supported `auth_method`, matching
SASL user/password and appropriate TLS settings; for Splunk supply an
HEC URL and token, and TLS settings as required. Add more blocks to test
multiple endpoints in one run. Use a separate fixture for explicit
format/condition or compression cases if desired. Keep credentials in
private files or the supported environment variables, never in the repo.

```bash
(
  set -o pipefail
  LEGACY_PROVIDER_VERSION=10.3.0 \
    MIGRATION_LOGGING_FIXTURE="$HOME/private/logging-test/legacy-blocks.hcl" \
    make test-migration-logging-tls
)
```

The script checks the initial plan creates only the isolated service, then
captures refreshed legacy state, runs analyze/dry-run/write, validates
target HCL, rejects any create/update/delete/replace/import migration action,
applies only the saved safe plan, and checks final no-op, exact state
(including all persisted logging credential fields), stable service ID and
active version, API methods and cleanup. State snapshots normalize only
target TLS/authentication object representation, empty/null auto-placement,
and zero target-only Splunk limits. A mismatch fails the test; do not
weaken the assertions merely to obtain a PASS.

The existing prefix sweep covers the test-owned service on failure. All raw
Terraform output, state, traces, plans and recovery files stay in a private
workspace. Only the phase, endpoint type/count summary and safe result are
printed. Failed runs retain the workspace: inspect locally, do not upload
the private logs or state. Set `KEEP_WORK_DIR=1` only if necessary and delete
the private artifacts manually after review.

Record actual SDKv2 versions, endpoint types and results in the PR. A PASS for
one endpoint is not evidence for another. The fixture is CDN-only; the
Compute schema/golden tests are separate. Do not claim live coverage of
receivers or credentials that are unavailable.
