---
page_title: "fastly_service_logging_fos Resource - fastly"
subcategory: ""
description: |-
  Fastly service Object Storage logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_fos (Resource)

Fastly service Object Storage logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a Fastly Object Storage real-time logging endpoint on the configured
service version. It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_fos` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_fos" "example" {
  service_id  = fastly_service_cdn.example.id
  version     = 1
  name        = "fos-example"
  bucket_name = "my-logs-bucket"
  region      = "us-east"
}
```

A fully configured endpoint. `format`, `format_version`, `placement`, and
`response_condition` only affect generated VCL, so they are valid when
`service_id` refers to a CDN (VCL) service and rejected for a Compute service:

```terraform
resource "fastly_service_logging_fos" "full" {
  service_id        = fastly_service_cdn.example.id
  version           = 1
  name              = "fos-full"
  bucket_name       = "my-logs-bucket"
  region            = "us-east"
  path              = "logs/"
  period            = 3600
  compression_codec = "zstd"
  file_max_bytes    = 104857600

  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
  response_condition = fastly_service_condition.errors_only.name
}
```

Attaching to a Compute service — the VCL-only attributes must be omitted:

```terraform
resource "fastly_service_logging_fos" "compute" {
  service_id  = fastly_service_compute.example.id
  version     = 1
  name        = "fos-compute"
  bucket_name = "my-logs-bucket"
  region      = "us-east"
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, Fastly Object Storage logging is a nested
block and the provider clones, validates, and activates a new service version
whenever the block changes. The nested block takes the same arguments as this
resource, minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  logging_fos {
    name        = "fos-example"
    bucket_name = "my-logs-bucket"
    region      = "us-east"
  }
}
```

`fastly_service_compute_auto` supports the same block, without the VCL-only
arguments (`format`, `format_version`, `placement`, `response_condition`):

```terraform
data "fastly_package_hash" "example" {
  filename = "package.tar.gz"
}

resource "fastly_service_compute_auto" "example" {
  name = "my-compute-service"

  package {
    filename         = "package.tar.gz"
    source_code_hash = data.fastly_package_hash.example.hash
  }

  logging_fos {
    name        = "fos-compute"
    bucket_name = "my-logs-bucket"
    region      = "us-east"
  }
}
```

## Schema

### Required

- `bucket_name` (String) The name of the Fastly Object Storage bucket.
- `name` (String) The name for the real-time logging configuration. Must be unique within the service.
- `region` (String) The region of the Fastly Object Storage bucket (e.g. `us-east`).
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `compression_codec` (String) The codec used for compressing your logs. Valid values are `zstd`, `snappy`, and `gzip`. If the codec is `gzip`, `gzip_level` defaults to `3`; to use a different level, leave `compression_codec` unset and set `gzip_level` instead. Conflicts with `gzip_level`: setting both in the same request will result in an error.
- `file_max_bytes` (Number) The maximum number of bytes for each uploaded file. A value of 0 can be used to indicate there is no limit on the size of uploaded files, otherwise the minimum value is 1048576 bytes (1 MiB.).
- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in vcl_log if format_version is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `gzip_level` (Number) The level of gzip encoding when sending logs. Valid values are `0` (no compression) through `9`. To compress at a specific gzip level, leave `compression_codec` unset and set this. Conflicts with `compression_codec`: setting both in the same request will result in an error.
- `message_type` (String) How the message should be formatted. Valid values are `classic`, `loggly`, `logplex`, and `blank`. Default `classic`.
- `path` (String) Path to store the files. Must end with a trailing slash. If this field is left empty, the files will be saved in the bucket's root path.
- `period` (Number) How frequently log files are finalized so they can be available for reading in seconds. Default `3600`.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with format_version of 2 are placed in vcl_log and those with format_version of 1 are placed in vcl_deliver. Valid value is `none`.
- `public_key` (String) PGP public key that Fastly will use to encrypt your log files before writing them to disk.
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- `timestamp_format` (String) strftime-specified timestamp format for log filename.

### Read-Only

- `id` (String) Terraform resource identifier.

## Import

For import-from-scratch with the Terraform CLI, include the service version in
the import ID so the provider can read the endpoint and
populate full state:

```shell
terraform import fastly_service_logging_fos.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_fos.example SU1Z0isxPaozGVKXdv0eY/3/fos-example
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.

## Notes

- This resource has no credential arguments. Access credentials for the
  endpoint are not configurable and are not exposed in Terraform state.
- Leaving `placement` unset is not the same as setting it to `none`: unset lets
  Fastly place the logging call automatically (`vcl_log` for `format_version` 2,
  `vcl_deliver` for `format_version` 1), while `none` suppresses the generated
  log statement entirely so you can write it yourself.
- `compression_codec` and `gzip_level` cannot be set together. To compress at a
  specific gzip level, leave `compression_codec` unset and set `gzip_level`.
