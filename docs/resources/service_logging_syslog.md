---
page_title: "fastly_service_logging_syslog Resource - fastly"
subcategory: ""
description: |-
  Fastly service Syslog logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_syslog (Resource)

Fastly service Syslog logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a Syslog real-time logging endpoint on the configured service
version. It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_syslog` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_syslog" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "syslog-example"

  address = "syslog.example.com"
  port    = 514
}
```

A fully configured endpoint using TLS. `format`, `format_version`, `placement`,
and `response_condition` only affect generated VCL, so they are valid when
`service_id` refers to a CDN (VCL) service and rejected for a Compute service:

```terraform
resource "fastly_service_logging_syslog" "tls" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "syslog-tls"

  address      = "syslog.example.com"
  port         = 6514
  message_type = "loggly"
  use_tls      = true

  authentication = {
    token = "a-token-to-prepend-to-each-message"
  }

  tls = {
    ca_cert  = file("ca-cert.pem")
    hostname = "syslog.example.com"
  }

  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
  response_condition = fastly_service_condition.errors_only.name
}
```

Attaching to a Compute service — the VCL-only attributes must be omitted:

```terraform
resource "fastly_service_logging_syslog" "compute" {
  service_id = fastly_service_compute.example.id
  version    = 1
  name       = "syslog-compute"

  address = "syslog.example.com"
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, Syslog logging is a nested block and
the provider clones, validates, and activates a new service version whenever
the block changes. The nested block takes the same arguments as this resource,
minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  domain {
    name = "www.example.com"
  }

  logging_syslog {
    name    = "syslog-example"
    address = "syslog.example.com"
  }
}
```

`fastly_service_compute_auto` supports the same block, without the VCL-only
arguments (`format`, `format_version`, `placement`, `response_condition`):

```terraform
resource "fastly_service_compute_auto" "example" {
  name = "my-compute-service"

  domain {
    name = "www.example.com"
  }

  package {
    filename         = "package.tar.gz"
    source_code_hash = filesha512("package.tar.gz")
  }

  logging_syslog {
    name    = "syslog-compute"
    address = "syslog.example.com"
  }
}
```

## Schema

### Required

- `address` (String) A hostname or IPv4 address of the Syslog endpoint.
- `name` (String) The name for the real-time logging configuration. Must be unique within the service.
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `authentication` (Attributes) Syslog authentication credentials. (see [below for nested schema](#nestedatt--authentication))
- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `message_type` (String) How the message should be formatted. Valid values are `classic`, `loggly`, `logplex`, and `blank`. Default `classic`.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of `2` are placed in `vcl_log` and those with `format_version` of `1` are placed in `vcl_deliver`. Valid value is `none`.
- `port` (Number) The port associated with the address where the Syslog endpoint can be accessed. Default `514`.
- `processing_region` (String) The geographic region where the logs will be processed before streaming. Valid values are `us`, `eu`, and `none` for global. Default: `none`.
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- `tls` (Attributes) TLS configuration used when `use_tls` is enabled. When this block is omitted entirely, `ca_cert`, `client_cert`, and `client_key` default to the `FASTLY_SYSLOG_CA_CERT`, `FASTLY_SYSLOG_CLIENT_CERT`, and `FASTLY_SYSLOG_CLIENT_KEY` environment variables. (see [below for nested schema](#nestedatt--tls))
- `use_tls` (Boolean) Whether to use TLS for secure logging. Default: `false`.

### Read-Only

- `id` (String) Terraform resource identifier.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Optional:

- `token` (String, Sensitive) Whether to prepend each message with a specific token.


<a id="nestedatt--tls"></a>
### Nested Schema for `tls`

Optional:

- `ca_cert` (String) A secure certificate to authenticate the server with. Must be in PEM format. Can be set via the `FASTLY_SYSLOG_CA_CERT` environment variable.
- `client_cert` (String) The client certificate used to make authenticated requests. Must be in PEM format. Can be set via the `FASTLY_SYSLOG_CLIENT_CERT` environment variable.
- `client_key` (String, Sensitive) The client private key used to make authenticated requests. Must be in PEM format. Can be set via the `FASTLY_SYSLOG_CLIENT_KEY` environment variable.
- `hostname` (String) The hostname used to verify the server's certificate. This should be one of the Subject Alternative Name (SAN) fields for the certificate. Common Names (CN) are not supported.

## Import

For import-from-scratch with the Terraform CLI, include the service version in
the import ID so the provider can read the endpoint from the Fastly API and
populate full state:

```shell
terraform import fastly_service_logging_syslog.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_syslog.example SU1Z0isxPaozGVKXdv0eY/3/syslog-example
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.

## Notes

- Leaving `placement` unset is not the same as setting it to `none`: unset lets
  Fastly place the logging call automatically (`vcl_log` for `format_version` 2,
  `vcl_deliver` for `format_version` 1), while `none` suppresses the generated
  log statement entirely so you can write it yourself.
- `message_type` controls how the log line is wrapped before being sent to the
  Syslog endpoint — `classic`, `loggly`, `logplex`, or `blank`.
- `authentication.token` and `tls.ca_cert`/`tls.client_cert`/`tls.client_key`
  are grouped into nested blocks to keep credential material out of top-level
  attributes. `tls.ca_cert`, `tls.client_cert`, and `tls.client_key` can also be
  set via the `FASTLY_SYSLOG_CA_CERT`, `FASTLY_SYSLOG_CLIENT_CERT`, and
  `FASTLY_SYSLOG_CLIENT_KEY` environment variables when the `tls` block is
  omitted entirely.
