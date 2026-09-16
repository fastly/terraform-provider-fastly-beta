---
page_title: "fastly_service_logging_honeycomb Resource - fastly"
subcategory: ""
description: |-
  Fastly service Honeycomb logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_honeycomb (Resource)

Fastly service Honeycomb logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a Honeycomb real-time logging endpoint on the configured service version.
It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_honeycomb` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_honeycomb" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "honeycomb-example"
  dataset    = "fastly_honeycomb"

  authentication = {
    token = var.honeycomb_write_key
  }
  processing_region = "us"
}
```

A fully configured endpoint. `format`, `format_version`, `placement`, and
`response_condition` only affect generated VCL, so they are valid when
`service_id` refers to a CDN (VCL) service and rejected for a Compute service:

```terraform
resource "fastly_service_logging_honeycomb" "eu" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "honeycomb-eu"
  dataset    = "fastly_honeycomb"

  authentication = {
    token = var.honeycomb_write_key
  }
  processing_region = "eu"

  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
  response_condition = fastly_service_condition.errors_only.name
}
```

Attaching to a Compute service — the VCL-only attributes must be omitted:

```terraform
resource "fastly_service_logging_honeycomb" "compute" {
  service_id = fastly_service_compute.example.id
  version    = 1
  name       = "honeycomb-compute"
  dataset    = "fastly_honeycomb"

  authentication = {
    token = var.honeycomb_write_key
  }
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, Honeycomb logging is a nested block and
the provider clones, validates, and activates a new service version whenever
the block changes. The nested block takes the same arguments as this resource,
minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  domain {
    name = "www.example.com"
  }

  logging_honeycomb {
    name    = "honeycomb-example"
    dataset = "fastly_honeycomb"
    authentication = {
      token = var.honeycomb_write_key
    }
    processing_region = "us"
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

  logging_honeycomb {
    name    = "honeycomb-compute"
    dataset = "fastly_honeycomb"
    authentication = {
      token = var.honeycomb_write_key
    }
  }
}
```

## Schema

### Required

- `authentication` (Attributes) Honeycomb authentication credentials. (see [below for nested schema](#nestedatt--authentication))
- `dataset` (String) The Honeycomb Dataset you want to log to.
- `name` (String) The name for the real-time logging configuration. Must be unique within the service.
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/). Must produce valid JSON that Honeycomb can ingest.
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if format_version is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with format_version of 2 are placed in vcl_log and those with format_version of 1 are placed in vcl_deliver. Valid value is `none`.
- `processing_region` (String) The geographic region where the logs will be processed before streaming. Valid values are `us`, `eu`, and `none` for global. Default: `none`.
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.

### Read-Only

- `id` (String) Terraform resource identifier.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Required:

- `token` (String, Sensitive) The Write Key from the Account page of your Honeycomb account.

## Import

For import-from-scratch with the Terraform CLI, include the service version in
the import ID so the provider can read the endpoint and
populate full state:

```shell
terraform import fastly_service_logging_honeycomb.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_honeycomb.example SU1Z0isxPaozGVKXdv0eY/3/honeycomb-example
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.

## Notes

- `authentication` groups credentials as the other logging endpoints do. It is
  required — there is no `FASTLY_HONEYCOMB_*` environment variable to default
  from. `token` is sensitive and never appears in plan output.
- Leaving `placement` unset is not the same as setting it to `none`: unset lets
  Fastly place the logging call automatically (`vcl_log` for `format_version` 2,
  `vcl_deliver` for `format_version` 1), while `none` suppresses the generated
  log statement entirely so you can write it yourself.
