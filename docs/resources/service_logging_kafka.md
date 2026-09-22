---
page_title: "fastly_service_logging_kafka Resource - fastly"
subcategory: ""
description: |-
  Fastly service Kafka logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_kafka (Resource)

Fastly service Kafka logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a Kafka real-time logging endpoint on the configured service
version. It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_kafka` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_kafka" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "kafka-example"

  brokers = "kafka-1.example.com:9092,kafka-2.example.com:9092"
  topic   = "logs"
}
```

A fully configured endpoint using SASL authentication and TLS. `format`,
`format_version`, `placement`, and `response_condition` only affect generated
VCL, so they are valid when `service_id` refers to a CDN (VCL) service and
rejected for a Compute service:

```terraform
resource "fastly_service_logging_kafka" "authenticated" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "kafka-authenticated"

  brokers           = "kafka-1.example.com:9092,kafka-2.example.com:9092"
  topic             = "logs"
  auth_method       = "scram-sha-512"
  compression_codec = "snappy"
  use_tls           = true

  authentication = {
    user     = "kafka-user"
    password = "a-strong-password"
  }

  tls = {
    ca_cert  = file("ca-cert.pem")
    hostname = "kafka-1.example.com"
  }

  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
  response_condition = fastly_service_condition.errors_only.name
}
```

Attaching to a Compute service — the VCL-only attributes must be omitted:

```terraform
resource "fastly_service_logging_kafka" "compute" {
  service_id = fastly_service_compute.example.id
  version    = 1
  name       = "kafka-compute"

  brokers = "kafka-1.example.com:9092"
  topic   = "logs"
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, Kafka logging is a nested block and
the provider clones, validates, and activates a new service version whenever
the block changes. The nested block takes the same arguments as this resource,
minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  logging_kafka {
    name    = "kafka-example"
    brokers = "kafka-1.example.com:9092,kafka-2.example.com:9092"
    topic   = "logs"
  }
}
```

`fastly_service_compute_auto` supports the same block, without the VCL-only
arguments (`format`, `format_version`, `placement`, `response_condition`):

```terraform
resource "fastly_service_compute_auto" "example" {
  name = "my-compute-service"

  package {
    filename         = "package.tar.gz"
    source_code_hash = filesha512("package.tar.gz")
  }

  logging_kafka {
    name    = "kafka-compute"
    brokers = "kafka-1.example.com:9092"
    topic   = "logs"
  }
}
```

## Schema

### Required

- `brokers` (String) A comma-separated list of IP addresses or hostnames of Kafka brokers.
- `name` (String) The name for the real-time logging configuration. Must be unique within the service.
- `service_id` (String) Fastly service ID.
- `topic` (String) The Kafka topic to send logs to.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `auth_method` (String) SASL authentication method. Valid values are `plain`, `scram-sha-256`, and `scram-sha-512`.
- `authentication` (Attributes) SASL authentication credentials. (see [below for nested schema](#nestedatt--authentication))
- `compression_codec` (String) The codec used for compression of your logs. Valid values are `gzip`, `snappy`, and `lz4`.
- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `parse_log_keyvals` (Boolean) Enables parsing of key=value tuples from the beginning of a logline, turning them into [record headers](https://cwiki.apache.org/confluence/display/KAFKA/KIP-82+-+Add+Record+Headers). Default `false`.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of `2` are placed in `vcl_log` and those with `format_version` of `1` are placed in `vcl_deliver`. Valid value is `none`.
- `processing_region` (String) The geographic region where the logs will be processed before streaming. Valid values are `us`, `eu`, and `none` for global. Default: `none`.
- `request_max_bytes` (Number) The maximum number of bytes sent in one request. Default `0` for no limit.
- `required_acks` (String) The number of acknowledgements a leader must receive before a write is considered successful. Valid values are `1` (one server needs to respond), `0` (no servers need to respond), and `-1` (wait for all in-sync replicas to respond). Default `1`.
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- `tls` (Attributes) TLS configuration used when `use_tls` is enabled. (see [below for nested schema](#nestedatt--tls))
- `use_tls` (Boolean) Whether to use TLS for secure logging. Default: `false`.

### Read-Only

- `id` (String) Terraform resource identifier.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Optional:

- `password` (String, Sensitive) SASL password.
- `user` (String) SASL user.


<a id="nestedatt--tls"></a>
### Nested Schema for `tls`

Optional:

- `ca_cert` (String) A secure certificate to authenticate the server with. Must be in PEM format.
- `client_cert` (String) The client certificate used to make authenticated requests. Must be in PEM format.
- `client_key` (String, Sensitive) The client private key used to make authenticated requests. Must be in PEM format.
- `hostname` (String) The hostname used to verify the server's certificate. This should be one of the Subject Alternative Name (SAN) fields for the certificate. Common Names (CN) are not supported.

## Import

For import-from-scratch with the Terraform CLI, include the service version in
the import ID so the provider can read the endpoint and
populate full state:

```shell
terraform import fastly_service_logging_kafka.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_kafka.example SU1Z0isxPaozGVKXdv0eY/3/kafka-example
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
- `auth_method` selects the SASL mechanism used with `authentication.user` and
  `authentication.password`. Leave it unset to connect without SASL
  authentication.
- `authentication.user`/`authentication.password` and
  `tls.ca_cert`/`tls.client_cert`/`tls.client_key` are grouped into nested
  blocks to keep credential material out of top-level attributes. Unlike some
  other logging endpoints, Kafka's credentials have no environment variable
  fallback.
