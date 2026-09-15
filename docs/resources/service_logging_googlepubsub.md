---
page_title: "fastly_service_logging_googlepubsub Resource - fastly"
subcategory: ""
description: |-
  Fastly service Google Cloud Pub/Sub logging endpoint resource. Writes directly to the specified writable service version.
---

# fastly_service_logging_googlepubsub (Resource)

Fastly service Google Cloud Pub/Sub logging endpoint resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a Pub/Sub real-time logging endpoint on the configured service
version. It does not clone, activate, or stage service versions.

To have the provider manage the version lifecycle for you instead, use the
nested `logging_googlepubsub` block on `fastly_service_cdn_auto` or
`fastly_service_compute_auto` — see "Automatic-lifecycle usage" below.

## Example Usage

```terraform
resource "fastly_service_logging_googlepubsub" "example" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "pubsub-example"

  project_id = "my-gcp-project"
  topic      = "my-topic"

  authentication = {
    email      = var.pubsub_service_account_email
    secret_key = var.pubsub_service_account_secret_key
  }

  format = "{\n \"timestamp\":\"%{begin:%Y-%m-%dT%H:%M:%S}t\",\n  \"client_ip\":\"%{req.http.Fastly-Client-IP}V\"\n}"
}
```

A fully configured endpoint, using `account_name` instead of `email`/`secret_key`
to reference a GCP service account already linked to the Fastly account.
`format`, `format_version`, `placement`, and `response_condition` only affect
generated VCL, so they are valid when `service_id` refers to a CDN (VCL)
service and rejected for a Compute service:

```terraform
resource "fastly_service_logging_googlepubsub" "linked_account" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "pubsub-linked-account"

  project_id = "my-gcp-project"
  topic      = "my-topic"

  authentication = {
    account_name = "my-linked-service-account"
  }
  processing_region = "eu"

  format             = "{\n \"timestamp\":\"%{begin:%Y-%m-%dT%H:%M:%S}t\"\n}"
  format_version     = 2
  placement          = "none"
  response_condition = fastly_service_condition.errors_only.name
}
```

Attaching to a Compute service — the VCL-only attributes must be omitted:

```terraform
resource "fastly_service_logging_googlepubsub" "compute" {
  service_id = fastly_service_compute.example.id
  version    = 1
  name       = "pubsub-compute"

  project_id = "my-gcp-project"
  topic      = "my-topic"

  authentication = {
    email      = var.pubsub_service_account_email
    secret_key = var.pubsub_service_account_secret_key
  }
}
```

## Automatic-lifecycle usage

Inside the `_auto` service resources, Pub/Sub logging is a nested block and
the provider clones, validates, and activates a new service version whenever
the block changes. The nested block takes the same arguments as this
resource, minus `service_id` and `version`, which the parent service owns.

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "my-service"

  domain {
    name = "www.example.com"
  }

  logging_googlepubsub {
    name       = "pubsub-example"
    project_id = "my-gcp-project"
    topic      = "my-topic"

    authentication = {
      email      = var.pubsub_service_account_email
      secret_key = var.pubsub_service_account_secret_key
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

  logging_googlepubsub {
    name       = "pubsub-compute"
    project_id = "my-gcp-project"
    topic      = "my-topic"

    authentication = {
      email      = var.pubsub_service_account_email
      secret_key = var.pubsub_service_account_secret_key
    }
  }
}
```

## Schema

### Required

- `name` (String) The name for the real-time logging configuration. Must be unique within the service.
- `project_id` (String) The ID of your Google Cloud Platform project.
- `service_id` (String) Fastly service ID.
- `topic` (String) The Google Cloud Pub/Sub topic to which logs will be published.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `authentication` (Attributes) Google Cloud Platform authentication credentials for Pub/Sub access. Provide either `account_name`, or `email` and `secret_key`. When this block is omitted entirely, defaults to the `FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME` (or `FASTLY_GCS_ACCOUNT_NAME`), `FASTLY_GOOGLE_PUBSUB_EMAIL`, and `FASTLY_GOOGLE_PUBSUB_SECRET_KEY` environment variables. (see [below for nested schema](#nestedatt--authentication))
- `format` (String) A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).
- `format_version` (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.
- `placement` (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of `2` are placed in `vcl_log` and those with `format_version` of `1` are placed in `vcl_deliver`. Valid value is `none`.
- `processing_region` (String) The geographic region where the logs will be processed before streaming to Google Cloud Pub/Sub. Valid values are `us`, `eu`, and `none` for global. Default: `none`.
- `response_condition` (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.

### Read-Only

- `id` (String) Terraform resource identifier.

<a id="nestedatt--authentication"></a>
### Nested Schema for `authentication`

Optional:

- `account_name` (String) The name of the Google Cloud Platform service account associated with the target log collection service. Not required if `email` and `secret_key` are provided. Can be set via the `FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME` environment variable (shared with Fastly's GCS and BigQuery logging endpoints), falling back to `FASTLY_GCS_ACCOUNT_NAME`.
- `email` (String, Sensitive) The `client_email` field in your service account authentication JSON. Not required if `account_name` is provided. Can be set via the `FASTLY_GOOGLE_PUBSUB_EMAIL` environment variable.
- `secret_key` (String, Sensitive) The `private_key` field in your service account authentication JSON. Not required if `account_name` is provided. Can be set via the `FASTLY_GOOGLE_PUBSUB_SECRET_KEY` environment variable.

## Import

For import-from-scratch with the Terraform CLI, include the service version in
the import ID so the provider can read the endpoint and
populate full state:

```shell
terraform import fastly_service_logging_googlepubsub.example SERVICE_ID/VERSION/ENDPOINT_NAME
```

Example:

```shell
terraform import fastly_service_logging_googlepubsub.example SU1Z0isxPaozGVKXdv0eY/3/pubsub-example
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use
explicit service-version lifecycle actions to clone, validate, stage, or
activate a service version.

## Notes

- `authentication` groups credentials as the other logging endpoints do.
  Provide either `account_name` (the name of a Google Cloud IAM service
  account set up for [impersonation](https://www.fastly.com/documentation/guides/integrations/streaming-logs/configuring-google-iam-service-account-impersonation-for-fastly-logging/)),
  or `email` and `secret_key`. When the block is omitted entirely, defaults to
  the `FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME`, `FASTLY_GOOGLE_PUBSUB_EMAIL`, and
  `FASTLY_GOOGLE_PUBSUB_SECRET_KEY` environment variables — `FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME`
  is shared with Fastly's GCS and BigQuery logging endpoints, since all three use
  the same Google Cloud service account. `email` and `secret_key` are
  sensitive and never appear in plan output. Once `account_name` is set, it
  can only be changed to a different value — not cleared back to unset —
  since an explicit empty `account_name` is rejected on update.
  `account_name` also falls back to the deprecated `FASTLY_GCS_ACCOUNT_NAME`
  environment variable when `FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME` is unset,
  emitting a deprecation warning so upgrading practitioners are not silently
  broken.
- `secret_key` must be a real PEM-encoded private key (PKCS8 or PKCS1) and
  must not contain leading or trailing whitespace — both are rejected.
- `project_id` is always required, unlike Fastly's GCS logging endpoint where
  it can be omitted in favor of `account_name`.
- If `format` is not sent, it falls back to a general JSON log format similar
  to the one used by other streaming-logs integrations.
- Leaving `placement` unset is not the same as setting it to `none`: unset
  lets Fastly place the logging call automatically (`vcl_log` for
  `format_version` 2, `vcl_deliver` for `format_version` 1), while `none`
  suppresses the generated log statement entirely so you can write it
  yourself.
