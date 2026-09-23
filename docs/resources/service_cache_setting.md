---
page_title: "fastly_service_cache_setting Resource - fastly"
subcategory: ""
description: |-
  Fastly service cache setting resource. Writes directly to the specified writable service version.
---

# fastly_service_cache_setting (Resource)

Fastly service cache setting resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a cache setting on the configured service version. It does not clone,
activate, or stage service versions. Cache settings are only supported on VCL
services.

## Example Usage

```terraform
resource "fastly_service_condition" "cache" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "cache_condition"
  type       = "CACHE"
  statement  = "beresp.status == 200"
}

resource "fastly_service_cache_setting" "example" {
  service_id      = fastly_service_cdn.example.id
  version         = 1
  name            = "example"
  action          = "cache"
  ttl             = 3600
  stale_ttl       = 120
  cache_condition = fastly_service_condition.cache.name
}
```

## Schema

### Required

- `name` (String) Unique name for this Cache Setting. Changing this attribute will delete and recreate the resource.
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `action` (String) One of `cache`, `pass`, or `restart`, as defined on Fastly's documentation under ["Caching action descriptions"](https://docs.fastly.com/en/guides/controlling-caching#caching-action-descriptions).
- `cache_condition` (String) Name of already defined `condition` used to test whether this settings object should be used. This `condition` must be of type `CACHE`.
- `stale_ttl` (Number) Max "Time To Live" (in seconds) for stale (unreachable) objects. Default `0`.
- `ttl` (Number) The Time-To-Live (TTL, in seconds) for the object. Default `0`.

### Read-Only

- `id` (String) Terraform resource identifier.

## Import

Import a cache setting using the service ID, version, and cache setting name:

```shell
terraform import fastly_service_cache_setting.example SERVICE_ID/VERSION/CACHE_SETTING_NAME
```

Example:

```shell
terraform import fastly_service_cache_setting.example SU1Z0isxPaozGVKXdv0eY/3/example
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.
