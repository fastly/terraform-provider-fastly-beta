---
page_title: "fastly_service_response_object Resource - fastly"
subcategory: ""
description: |-
  Fastly service response object resource. Writes directly to the specified writable service version.
---

# fastly_service_response_object (Resource)

Fastly service response object resource. Writes directly to the specified writable service version.

This resource is part of the explicit/default first-class resource family. It
manages a Response Object on the configured service version. It does not clone,
activate, or stage service versions. Response Objects are supported on CDN
services only.

## Example Usage

```terraform
resource "fastly_service_condition" "request" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "maintenance-request"
  type       = "REQUEST"
  statement  = "req.url ~ \"^/maintenance\""
}

resource "fastly_service_condition" "cache" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "maintenance-cache"
  type       = "CACHE"
  statement  = "beresp.status == 503"
}

resource "fastly_service_response_object" "maintenance" {
  service_id = fastly_service_cdn.example.id
  version    = 1
  name       = "maintenance-response"

  status            = 503
  response          = "Service Unavailable"
  content           = "Temporarily unavailable"
  content_type      = "text/plain"
  request_condition = fastly_service_condition.request.name
  cache_condition   = fastly_service_condition.cache.name
}
```

## Schema

### Required

- `name` (String) A unique name to identify this Response Object. Changing this attribute will delete and recreate the resource.
- `service_id` (String) Fastly service ID.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `cache_condition` (String) Name of already defined `condition` to check after we have retrieved an object. If the condition passes then deliver this Response Object instead. This `condition` must be of type `CACHE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals](https://docs.fastly.com/en/guides/using-conditions)
- `content` (String) The content to deliver for the response object.
- `content_type` (String) The MIME type of the content, can be empty.
- `request_condition` (String) Name of already defined `condition` to be checked during the request phase. If the condition passes then this object will be delivered. This `condition` must be of type `REQUEST`.
- `response` (String) The HTTP Response. Default `OK`.
- `status` (Number) The HTTP Status Code. Default `200`.

### Read-Only

- `id` (String) Terraform resource identifier.

## Import

Import a Response Object using the service ID, version, and Response Object name:

```shell
terraform import fastly_service_response_object.maintenance SERVICE_ID/VERSION/RESPONSE_OBJECT_NAME
```

Example:

```shell
terraform import fastly_service_response_object.maintenance SU1Z0isxPaozGVKXdv0eY/3/maintenance-response
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.
