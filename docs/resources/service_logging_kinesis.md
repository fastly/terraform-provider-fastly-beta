---
page_title: "fastly_service_logging_kinesis Resource - fastly"
subcategory: ""
description: |-
  Fastly service Kinesis logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_kinesis (Resource)

Fastly service Kinesis logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a Kinesis real-time logging endpoint on the configured service
version. It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_kinesis` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_kinesis" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "kinesis-example"

  topic = "my-kinesis-stream"

  authentication = {
    access_key = "AKIAIOSFODNN7EXAMPLE"
    secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  }
}
```

A fully configured endpoint using explicit AWS credentials. `format`,
`format_version`, `placement`, and `response_condition` only affect generated
VCL, so they are valid when `service_id` refers to a CDN (VCL) service and
rejected for a Compute service:

```terraform
resource "fastly_service_logging_kinesis" "authenticated" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "kinesis-authenticated"

  topic             = "my-kinesis-stream"
  region            = "us-west-2"
  processing_region = "us"

  authentication = {
    access_key = "AKIAIOSFODNN7EXAMPLE"
    secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  }

  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
  response_condition = fastly_service_condition.errors_only.name
}
```

Attaching to a Compute service — the VCL-only attributes must be omitted:

```terraform
resource "fastly_service_logging_kinesis" "compute" {
  service_id = fastly_service_compute.example.id
  version    = 1
  name       = "kinesis-compute"

  topic = "my-kinesis-stream"

  authentication = {
    access_key = "AKIAIOSFODNN7EXAMPLE"
    secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  }
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, Kinesis logging is a nested block and
the provider clones, validates, and activates a new service version whenever
the block changes. The nested block takes the same arguments as this resource,
minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  domain {
    name = "www.example.com"
  }

  logging_kinesis {
    name  = "kinesis-example"
    topic = "my-kinesis-stream"

    authentication = {
      access_key = "AKIAIOSFODNN7EXAMPLE"
      secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    }
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

  logging_kinesis {
    name  = "kinesis-compute"
    topic = "my-kinesis-stream"

    authentication = {
      access_key = "AKIAIOSFODNN7EXAMPLE"
      secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    }
  }
}
```

## Schema

### Required

- `name` (String) The unique name of the Kinesis logging endpoint. It is important to note that changing this attribute will delete and recreate the resource.
- `service_id` (String) Fastly service ID.
- `topic` (String) The Kinesis stream name.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `authentication` (Attributes) AWS authentication credentials for the Kinesis stream. Provide either `access_key` and `secret_key`, or `iam_role`. (see [below for nested schema](#nestedatt--authentication))
- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of `2` are placed in `vcl_log` and those with `format_version` of `1` are placed in `vcl_deliver`. Valid value is `none`.
- `processing_region` (String) Region where logs will be processed before streaming to the destination. Valid values are `none`, `us` and `eu`.
- `region` (String) The AWS region the stream resides in. Default `us-east-1`.
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.

### Read-Only

- `id` (String) Terraform resource identifier.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Optional:

- `access_key` (String, Sensitive) The AWS access key to be used to write to the stream. Not required if `iam_role` is provided.
- `iam_role` (String) The Amazon Resource Name (ARN) for the IAM role granting Fastly access to Kinesis. Not required if `access_key` and `secret_key` are provided.
- `secret_key` (String, Sensitive) The AWS secret access key to authenticate with. Not required if `iam_role` is provided.

## Import

For import-from-scratch with the Terraform CLI, include the service version in
the import ID so the provider can read the endpoint and
populate full state:

```shell
terraform import fastly_service_logging_kinesis.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_kinesis.example SU1Z0isxPaozGVKXdv0eY/3/kinesis-example
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
- `authentication.access_key`/`authentication.secret_key` and
  `authentication.iam_role` are alternative ways to authenticate to the target
  Kinesis stream — provide either the access/secret key pair or an IAM role.
  Unlike some other logging endpoints, Kinesis's credentials have no
  environment variable fallback, and unlike S3, the `authentication` block
  can't be omitted entirely: the Fastly API rejects a Kinesis endpoint with
  none of the three credentials set, so this provider enforces the same rule
  at plan time.
