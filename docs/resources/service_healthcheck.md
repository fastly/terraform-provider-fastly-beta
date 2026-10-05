---
page_title: "fastly_service_healthcheck Resource - fastly"
subcategory: ""
description: |-
  Fastly service health check resource. Writes directly to the specified writable service version.
---

# fastly_service_healthcheck (Resource)

Fastly service health check resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a health check on the configured service version. It does not clone,
activate, or stage service versions. Health checks are supported on both CDN
and Compute services.

## Example Usage

```terraform
resource "fastly_service_healthcheck" "origin" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "origin-healthcheck"
  host       = "api.example.com"
  path       = "/healthz"

  check_interval    = 10000
  expected_response = 200
  method            = "GET"
  headers           = ["X-Api-Key: abc123"]
}

resource "fastly_service_backend" "origin" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "origin"

  address     = "api.example.com"
  port        = 443
  use_ssl     = true
  healthcheck = fastly_service_healthcheck.origin.name
}
```

## Schema

### Required

- `host` (String) The host to check.
- `name` (String) A unique name to identify this health check. Changing this attribute will delete and recreate the resource.
- `path` (String) The path to check.
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `check_interval` (Number) How often to run the health check in milliseconds. Must be between `1000` and `3600000`. Default `5000`.
- `expected_response` (Number) The status code expected from the host. Default `200`.
- `headers` (Set of String) Custom health check HTTP headers (e.g. if your health check requires an API key to be provided).
- `http_version` (String) Whether to use version `1.0` or `1.1` HTTP. Default `1.1`.
- `initial` (Number) When loading a config, the initial number of probes to be seen as OK. Default `3`.
- `method` (String) Which HTTP method to use. Default `HEAD`.
- `threshold` (Number) How many health checks must succeed to be considered healthy. Default `3`.
- `timeout` (Number) Timeout in milliseconds. Default `5000`.
- `window` (Number) The number of most recent health check queries to keep for this health check. Default `5`.

### Read-Only

- `id` (String) Terraform resource identifier.

## Import

Import a health check using the service ID, version, and health check name:

```shell
terraform import fastly_service_healthcheck.origin SERVICE_ID/VERSION/HEALTHCHECK_NAME
```

Example:

```shell
terraform import fastly_service_healthcheck.origin SU1Z0isxPaozGVKXdv0eY/3/origin-healthcheck
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.

