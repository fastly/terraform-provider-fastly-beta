---
page_title: "fastly_service_logging_cloudfiles Resource - fastly"
subcategory: ""
description: |-
  Fastly service Cloud Files logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_cloudfiles (Resource)

Fastly service Cloud Files logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a Rackspace Cloud Files real-time logging endpoint on the configured
service version. It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_cloudfiles` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_cloudfiles" "example" {
  service_id  = fastly_service_cdn.example.id
  version     = 1
  name        = "cloudfiles-example"
  bucket_name = "my-logs-container"

  authentication = {
    user       = var.cloudfiles_user
    access_key = var.cloudfiles_access_key
  }
  region = "ORD"
}
```

A fully configured endpoint. `format`, `format_version`, `placement`, and
`response_condition` only affect generated VCL, so they are valid when
`service_id` refers to a CDN (VCL) service and rejected for a Compute service:

```terraform
resource "fastly_service_logging_cloudfiles" "full" {
  service_id  = fastly_service_cdn.example.id
  version     = 1
  name        = "cloudfiles-full"
  bucket_name = "my-logs-container"

  authentication = {
    user       = var.cloudfiles_user
    access_key = var.cloudfiles_access_key
  }
  region            = "LON"
  path              = "/logs/"
  period            = 3600
  compression_codec = "gzip"
  processing_region = "eu"

  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
  response_condition = fastly_service_condition.errors_only.name
}
```

Attaching to a Compute service — the VCL-only attributes must be omitted:

```terraform
resource "fastly_service_logging_cloudfiles" "compute" {
  service_id  = fastly_service_compute.example.id
  version     = 1
  name        = "cloudfiles-compute"
  bucket_name = "my-logs-container"

  authentication = {
    user       = var.cloudfiles_user
    access_key = var.cloudfiles_access_key
  }
  region = "ORD"
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, Cloud Files logging is a nested block and
the provider clones, validates, and activates a new service version whenever
the block changes. The nested block takes the same arguments as this resource,
minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  domain {
    name = "www.example.com"
  }

  logging_cloudfiles {
    name        = "cloudfiles-example"
    bucket_name = "my-logs-container"
    authentication = {
      user       = var.cloudfiles_user
      access_key = var.cloudfiles_access_key
    }
    region = "ORD"
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

  logging_cloudfiles {
    name        = "cloudfiles-compute"
    bucket_name = "my-logs-container"
    authentication = {
      user       = var.cloudfiles_user
      access_key = var.cloudfiles_access_key
    }
    region = "ORD"
  }
}
```

## Schema

### Required

- `authentication` (Attributes) Authentication credentials for your Rackspace Cloud Files account. (see [below for nested schema](#nestedatt--authentication))
- `bucket_name` (String) The name of your Cloud Files container.
- `name` (String) The name for the real-time logging configuration. Must be unique within the service.
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `compression_codec` (String) The codec used for compressing your logs. Valid values are `zstd`, `snappy`, and `gzip`. If the codec is `gzip`, `gzip_level` defaults to `3`; to use a different level, leave `compression_codec` unset and set `gzip_level` instead. Conflicts with `gzip_level`: setting both in the same request will result in an error.
- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in vcl_log if format_version is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `gzip_level` (Number) The level of gzip encoding when sending logs. Valid values are `0` (no compression) through `9`. To compress at a specific gzip level, leave `compression_codec` unset and set this. Conflicts with `compression_codec`: setting both in the same request will result in an error.
- `message_type` (String) How the message should be formatted. Valid values are `classic`, `loggly`, `logplex`, and `blank`. Default `blank`.
- `path` (String) The path to upload logs to.
- `period` (Number) How frequently log files are finalized so they can be available for reading in seconds. Default `3600`.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with format_version of 2 are placed in vcl_log and those with format_version of 1 are placed in vcl_deliver. Valid value is `none`.
- `processing_region` (String) Region where logs will be processed before streaming to the destination. Valid values are `none`, `us` and `eu`.
- `public_key` (String) PGP public key that Fastly will use to encrypt your log files before writing them to disk.
- `region` (String) The region to stream logs to. One of: `DFW` (Dallas), `ORD` (Chicago), `IAD` (Northern Virginia), `LON` (London), `SYD` (Sydney), `HKG` (Hong Kong).
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- `timestamp_format` (String) strftime-specified timestamp format for log filename.

### Read-Only

- `id` (String) Terraform resource identifier.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Required:

- `access_key` (String, Sensitive) Your Cloud Files account access key.
- `user` (String) The username for your Cloud Files account.

## Import

For import-from-scratch with the Terraform CLI, include the service version in
the import ID so the provider can read the endpoint and
populate full state:

```shell
terraform import fastly_service_logging_cloudfiles.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_cloudfiles.example SU1Z0isxPaozGVKXdv0eY/3/cloudfiles-example
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.

## Notes

- `authentication` groups credentials as the other logging endpoints do. It is
  required — there is no `FASTLY_CLOUDFILES_*` environment variable to default
  from. `access_key` is sensitive and never appears in plan output.
- Leaving `placement` unset is not the same as setting it to `none`: unset lets
  Fastly place the logging call automatically (`vcl_log` for `format_version` 2,
  `vcl_deliver` for `format_version` 1), while `none` suppresses the generated
  log statement entirely so you can write it yourself.
- `region` accepts `DFW`, `ORD`, `IAD`, `LON`, `SYD`, and `HKG`.
