---
page_title: "fastly_service_request_setting Resource - fastly"
subcategory: ""
description: |-
  Fastly service request setting resource. Writes directly to the specified writable service version.
---

# fastly_service_request_setting (Resource)

Fastly service request setting resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a Request Setting object on the configured service version. It does not
clone, activate, or stage service versions. Request Settings are supported on
CDN services only.

## Example Usage

```terraform
resource "fastly_service_condition" "api" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "api-requests"
  type       = "REQUEST"
  statement  = "req.url ~ \"^/api/\""
}

resource "fastly_service_request_setting" "api" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "api-request-settings"

  action            = "pass"
  request_condition = fastly_service_condition.api.name
  force_ssl         = true
  xff               = "append"
}
```

## Schema

### Required

- `name` (String) Unique name to refer to this Request Setting. Changing this attribute will delete and recreate the resource.
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `action` (String) Allows you to terminate request handling and immediately perform an action. When set it can be `lookup` or `pass` (ignore the cache completely).
- `bypass_busy_wait` (Boolean) Disable collapsed forwarding, so you don't wait for other objects to origin. Default `false`.
- `default_host` (String) Sets the host header.
- `force_miss` (Boolean) Force a cache miss for the request. Default `false`.
- `force_ssl` (Boolean) Forces the request to use SSL (redirects a non-SSL request to SSL). Default `false`.
- `hash_keys` (String) Comma separated list of varnish request object fields that should be in the hash key.
- `max_stale_age` (Number) How old an object is allowed to be to serve `stale-if-error` or `stale-while-revalidate`, in seconds. Default `0`.
- `request_condition` (String) Name of already defined `condition` to determine if this request setting should be applied. Should be unique across multiple instances of `request_setting`, including any left unset (Fastly rejects more than one request setting sharing the same, or unset, `request_condition`).
- `timer_support` (Boolean) Injects the X-Timer info into the request for viewing origin fetch durations. Default `false`.
- `xff` (String) X-Forwarded-For, should be `clear`, `leave`, `append`, `append_all`, or `overwrite`.

### Read-Only

- `id` (String) Terraform resource identifier.

## Import

Import a Request Setting using the service ID, version, and Request Setting name:

```shell
terraform import fastly_service_request_setting.api SERVICE_ID/VERSION/REQUEST_SETTING_NAME
```

Example:

```shell
terraform import fastly_service_request_setting.api SU1Z0isxPaozGVKXdv0eY/3/api-request-settings
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.
