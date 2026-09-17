---
page_title: "fastly_service_logging_sftp Resource - fastly"
subcategory: ""
description: |-
  Fastly service SFTP logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_sftp (Resource)

Fastly service SFTP logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages an SFTP real-time logging endpoint on the configured service version.
It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_sftp` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_sftp" "example" {
  service_id      = fastly_service_cdn.example.id
  version         = 1
  name            = "sftp-example"
  address         = "sftp.example.com"
  path            = "/logs/"
  ssh_known_hosts = var.sftp_ssh_known_hosts

  authentication = {
    user     = var.sftp_user
    password = var.sftp_password
  }
}
```

A fully configured endpoint, authenticating with an SSH private key instead of
a password. `format`, `format_version`, `placement`, and `response_condition`
only affect generated VCL, so they are valid when `service_id` refers to a CDN
(VCL) service and rejected for a Compute service:

```terraform
resource "fastly_service_logging_sftp" "full" {
  service_id      = fastly_service_cdn.example.id
  version         = 1
  name            = "sftp-full"
  address         = "sftp.example.com"
  path            = "/logs/"
  ssh_known_hosts = var.sftp_ssh_known_hosts
  port            = 22

  authentication = {
    user       = var.sftp_user
    secret_key = var.sftp_secret_key
  }
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
resource "fastly_service_logging_sftp" "compute" {
  service_id      = fastly_service_compute.example.id
  version         = 1
  name            = "sftp-compute"
  address         = "sftp.example.com"
  path            = "/logs/"
  ssh_known_hosts = var.sftp_ssh_known_hosts

  authentication = {
    user     = var.sftp_user
    password = var.sftp_password
  }
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, SFTP logging is a nested block and the
provider clones, validates, and activates a new service version whenever the
block changes. The nested block takes the same arguments as this resource,
minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  domain {
    name = "www.example.com"
  }

  logging_sftp {
    name            = "sftp-example"
    address         = "sftp.example.com"
    path            = "/logs/"
    ssh_known_hosts = var.sftp_ssh_known_hosts
    authentication = {
      user     = var.sftp_user
      password = var.sftp_password
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

  logging_sftp {
    name            = "sftp-compute"
    address         = "sftp.example.com"
    path            = "/logs/"
    ssh_known_hosts = var.sftp_ssh_known_hosts
    authentication = {
      user     = var.sftp_user
      password = var.sftp_password
    }
  }
}
```

## Schema

### Required

- `address` (String) A hostname or IPv4 address of the SFTP server.
- `authentication` (Attributes) Authentication credentials for the SFTP server. Exactly one of `password` or `secret_key` must be set; if both are set, `secret_key` is preferred by the API. (see [below for nested schema](#nestedatt--authentication))
- `name` (String) The name for the real-time logging configuration. Must be unique within the service.
- `path` (String) The path to upload log files to. If the path ends in `/` then it is treated as a directory.
- `service_id` (String) Fastly service ID.
- `ssh_known_hosts` (String) A list of host keys for all hosts we can connect to over SFTP.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `compression_codec` (String) The codec used for compressing your logs. Valid values are `zstd`, `snappy`, and `gzip`. If the codec is `gzip`, `gzip_level` defaults to `3`; to use a different level, leave `compression_codec` unset and set `gzip_level` instead. Conflicts with `gzip_level`: setting both in the same request will result in an error.
- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `gzip_level` (Number) The level of gzip encoding when sending logs. Valid values are `0` (no compression) through `9`. To compress at a specific gzip level, leave `compression_codec` unset and set this. Conflicts with `compression_codec`: setting both in the same request will result in an error.
- `message_type` (String) How the message should be formatted. Valid values are `classic`, `loggly`, `logplex`, and `blank`. Default `classic`.
- `period` (Number) How frequently log files are finalized so they can be available for reading, in seconds. Default `3600`.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of `2` are placed in `vcl_log` and those with `format_version` of `1` are placed in `vcl_deliver`. Valid value is `none`.
- `port` (Number) The port number. Default `22`.
- `processing_region` (String) The geographic region where the logs will be processed before streaming. Valid values are `none`, `us` and `eu`.
- `public_key` (String) PGP public key that Fastly will use to encrypt your log files before writing them to disk.
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- `timestamp_format` (String) A strftime-specified timestamp format for log filenames.

### Read-Only

- `id` (String) Terraform resource identifier.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Required:

- `user` (String) The username for the server.

Optional:

- `password` (String, Sensitive) The password for the server. If both `password` and `secret_key` are set, `secret_key` is preferred.
- `secret_key` (String, Sensitive) The SSH private key for the server. If both `password` and `secret_key` are set, `secret_key` is preferred.

## Import

For import-from-scratch with the Terraform CLI, include the service version in
the import ID so the provider can read the endpoint and
populate full state:

```shell
terraform import fastly_service_logging_sftp.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_sftp.example SU1Z0isxPaozGVKXdv0eY/3/sftp-example
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.

## Notes

- `authentication` groups credentials as the other logging endpoints do. It is
  required — there is no `FASTLY_SFTP_*` environment variable to default from.
  Exactly one of `password` or `secret_key` must be set; if both are set, the
  API prefers `secret_key`. Both are sensitive and never appear in plan output.
- `ssh_known_hosts` is required: a list of host keys for the hosts Terraform is
  allowed to connect to over SFTP.
- Leaving `placement` unset is not the same as setting it to `none`: unset lets
  Fastly place the logging call automatically (`vcl_log` for `format_version` 2,
  `vcl_deliver` for `format_version` 1), while `none` suppresses the generated
  log statement entirely so you can write it yourself.
- `port` defaults to `22`.
