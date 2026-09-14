---
page_title: "fastly_service_logging_elasticsearch Resource - fastly"
subcategory: ""
description: |-
  Fastly service Elasticsearch logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_elasticsearch (Resource)

Fastly service Elasticsearch logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages an Elasticsearch real-time logging endpoint on the configured service
version. It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_elasticsearch` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_elasticsearch" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "elasticsearch-example"

  index = "logs-index"
  url   = "https://elasticsearch.example.com"
}
```

A fully configured endpoint using BasicAuth and TLS. `format`,
`format_version`, `placement`, and `response_condition` only affect generated
VCL, so they are valid when `service_id` refers to a CDN (VCL) service and
rejected for a Compute service:

```terraform
resource "fastly_service_logging_elasticsearch" "tls" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "elasticsearch-tls"

  index    = "logs-index"
  url      = "https://elasticsearch.example.com"
  pipeline = "my-pipeline"

  authentication = {
    user     = "elastic"
    password = "a-basic-auth-password"
  }

  tls = {
    ca_cert  = file("ca-cert.pem")
    hostname = "elasticsearch.example.com"
  }

  processing_region   = "us"
  request_max_bytes   = 1000000
  request_max_entries = 1000

  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
  response_condition = fastly_service_condition.errors_only.name
}
```

Attaching to a Compute service — the VCL-only attributes must be omitted:

```terraform
resource "fastly_service_logging_elasticsearch" "compute" {
  service_id = fastly_service_compute.example.id
  version    = 1
  name       = "elasticsearch-compute"

  index = "logs-index"
  url   = "https://elasticsearch.example.com"
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, Elasticsearch logging is a nested block
and the provider clones, validates, and activates a new service version
whenever the block changes. The nested block takes the same arguments as this
resource, minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  domain {
    name = "www.example.com"
  }

  logging_elasticsearch {
    name  = "elasticsearch-example"
    index = "logs-index"
    url   = "https://elasticsearch.example.com"
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

  logging_elasticsearch {
    name  = "elasticsearch-compute"
    index = "logs-index"
    url   = "https://elasticsearch.example.com"
  }
}
```

## Schema

### Required

- `index` (String) The name of the Elasticsearch index to send documents (logs) to.
- `name` (String) The unique name of the Elasticsearch logging endpoint.
- `service_id` (String) Fastly service ID.
- `url` (String) The Elasticsearch URL to stream logs to. Must use HTTPS.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `authentication` (Attributes) BasicAuth credentials for Elasticsearch. (see [below for nested schema](#nestedatt--authentication))
- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `pipeline` (String) The ID of the Elasticsearch ingest pipeline to apply pre-process transformations to before indexing.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of `2` are placed in `vcl_log` and those with `format_version` of `1` are placed in `vcl_deliver`. Valid value is `none`.
- `processing_region` (String) The geographic region where the logs will be processed before streaming. Valid values are `us`, `eu`, and `none` for global. Default: `none`.
- `request_max_bytes` (Number) The maximum number of bytes sent in one request. Default `0` for unbounded.
- `request_max_entries` (Number) The maximum number of logs sent in one request. Default `0` for unbounded.
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- `tls` (Attributes) TLS configuration for the Elasticsearch endpoint. (see [below for nested schema](#nestedatt--tls))

### Read-Only

- `id` (String) Terraform resource identifier.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Optional:

- `password` (String, Sensitive) BasicAuth password for Elasticsearch.
- `user` (String) BasicAuth username for Elasticsearch.


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
terraform import fastly_service_logging_elasticsearch.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_elasticsearch.example SU1Z0isxPaozGVKXdv0eY/3/elasticsearch-example
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
- `authentication.user`/`authentication.password` and
  `tls.ca_cert`/`tls.client_cert`/`tls.client_key` are grouped into nested
  blocks to keep credential material out of top-level attributes. Unlike some
  other logging endpoints, none of these have an environment variable
  fallback.
- `tls.ca_cert`, `tls.client_cert`, and `tls.client_key` must not contain
  leading or trailing whitespace (e.g. a trailing newline from `file()`); wrap
  the value in `trimspace()` if needed.
- `request_max_bytes` and `request_max_entries` both default to `0`, meaning
  unbounded.
