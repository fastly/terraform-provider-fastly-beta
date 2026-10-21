---
page_title: "fastly_service_logging_blobstorage Resource - fastly"
subcategory: ""
description: |-
  Fastly service Blob Storage logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_blobstorage (Resource)

Fastly service Blob Storage logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages an Azure Blob Storage real-time logging endpoint on the configured
service version. It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_blobstorage` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_blobstorage" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "blobstorage-example"
  container  = "my-log-container"

  authentication = {
    account_name = var.azure_storage_account_name
    sas_token    = var.azure_storage_sas_token
  }
}
```

A fully configured endpoint. `format`, `format_version`, `placement`, and
`response_condition` only affect generated VCL, so they are valid when
`service_id` refers to a CDN (VCL) service and rejected for a Compute service:

```terraform
resource "fastly_service_logging_blobstorage" "full" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "blobstorage-full"
  container  = "my-log-container"
  path       = "/logs/"
  period     = 1800

  authentication = {
    account_name = var.azure_storage_account_name
    sas_token    = var.azure_storage_sas_token
  }
  processing_region = "eu"
  compression_codec = "gzip"
  message_type      = "loggly"

  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
  response_condition = fastly_service_condition.errors_only.name
}
```

Attaching to a Compute service — the VCL-only attributes must be omitted:

```terraform
resource "fastly_service_logging_blobstorage" "compute" {
  service_id = fastly_service_compute.example.id
  version    = 1
  name       = "blobstorage-compute"
  container  = "my-log-container"

  authentication = {
    account_name = var.azure_storage_account_name
    sas_token    = var.azure_storage_sas_token
  }
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, Blob Storage logging is a nested block
and the provider clones, validates, and activates a new service version
whenever the block changes. The nested block takes the same arguments as this
resource, minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  logging_blobstorage {
    name      = "blobstorage-example"
    container = "my-log-container"

    authentication = {
      account_name = var.azure_storage_account_name
      sas_token    = var.azure_storage_sas_token
    }
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

  logging_blobstorage {
    name      = "blobstorage-compute"
    container = "my-log-container"

    authentication = {
      account_name = var.azure_storage_account_name
      sas_token    = var.azure_storage_sas_token
    }
  }
}
```

## Schema

### Required

- `container` (String) The name of the Azure Blob Storage container in which to store logs.
- `name` (String) The name for the real-time logging configuration. Must be unique within the service.
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `authentication` (Attributes) Azure authentication credentials for Blob Storage access. Both `account_name` and `sas_token` are required. When this block is omitted entirely, defaults to the `FASTLY_AZURE_ACCOUNT_NAME` and `FASTLY_AZURE_SHARED_ACCESS_SIGNATURE` environment variables. (see [below for nested schema](#nestedatt--authentication))
- `compression_codec` (String) The codec used for compressing your logs. Valid values are `zstd`, `snappy`, and `gzip`. If the codec is `gzip`, `gzip_level` defaults to `3`; to use a different level, leave `compression_codec` unset and set `gzip_level` instead. Conflicts with `gzip_level`: setting both in the same request will result in an error.
- `file_max_bytes` (Number) The maximum number of bytes for each uploaded file. A value of `0` can be used to indicate there is no limit on the size of uploaded files, otherwise the minimum value is `1048576` bytes (1 MiB).
- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `gzip_level` (Number) The level of gzip encoding when sending logs. Valid values are `0` (no compression) through `9`. To compress at a specific gzip level, leave `compression_codec` unset and set this. Conflicts with `compression_codec`: setting both in the same request will result in an error.
- `message_type` (String) How the message should be formatted. Valid values are `classic`, `loggly`, `logplex`, and `blank`. Default `classic`.
- `path` (String) The path to upload logs to. Must end with a trailing slash. If this field is left empty, the files will be saved in the container's root path.
- `period` (Number) How frequently log files are finalized so they can be available for reading in seconds. Default `3600`.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of `2` are placed in `vcl_log` and those with `format_version` of `1` are placed in `vcl_deliver`. Valid value is `none`.
- `processing_region` (String) Region where logs will be processed before streaming to the destination. Valid values are `none`, `us` and `eu`.
- `public_key` (String) A PGP public key that Fastly will use to encrypt your log files before writing them to disk.
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- `timestamp_format` (String) `strftime`-specified timestamp format for log filename.

### Read-Only

- `id` (String) Terraform resource identifier.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Optional:

- `account_name` (String) The unique Azure Blob Storage namespace in which your data objects are stored. Can be set via the `FASTLY_AZURE_ACCOUNT_NAME` environment variable.
- `sas_token` (String, Sensitive) The Azure shared access signature providing write access to the blob service objects. Be sure to update your token before it expires or the logging functionality will not work. Can be set via the `FASTLY_AZURE_SHARED_ACCESS_SIGNATURE` environment variable.

## Import

For import-from-scratch with the Terraform CLI, include the service version in
the import ID so the provider can read the endpoint and
populate full state:

```shell
terraform import fastly_service_logging_blobstorage.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_blobstorage.example SU1Z0isxPaozGVKXdv0eY/3/blobstorage-example
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use
explicit service-version lifecycle actions to clone, validate, stage, or
activate a service version.

## Notes

- `authentication` requires both `account_name` and `sas_token` — unlike some
  other logging endpoints, Blob Storage has no alternate credential
  combination. When the block is omitted entirely, defaults to the
  `FASTLY_AZURE_ACCOUNT_NAME` and `FASTLY_AZURE_SHARED_ACCESS_SIGNATURE`
  environment variables. `sas_token` is sensitive and never appears in plan
  output; be sure to update it before it expires.
- `compression_codec` and `gzip_level` are mutually exclusive; setting both
  is rejected.
- Leaving `placement` unset is not the same as setting it to `none`: unset
  lets Fastly place the logging call automatically (`vcl_log` for
  `format_version` 2, `vcl_deliver` for `format_version` 1), while `none`
  suppresses the generated log statement entirely so you can write it
  yourself.
