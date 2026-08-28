---
page_title: "fastly_api_security_operation_tag Resource - fastly"
subcategory: ""
description: |-
  Manages an API Security operation tag for a Fastly service.
---

# fastly_api_security_operation_tag (Resource)

Manages an API Security operation tag for a Fastly service. Operation tags can be used to group and organize operations.

## Example Usage

```terraform
resource "fastly_service_cdn" "example" {
  name = "example-service"

  domain {
    name = "example.com"
  }

  backend {
    address = "http-me.fastly.dev"
    name    = "backend"
  }
}

resource "fastly_api_security_operation_tag" "example" {
  service_id  = fastly_service_cdn.example.id
  name        = "production"
  description = "Tag for production endpoints"
}

resource "fastly_api_security_operation" "example" {
  service_id  = fastly_service_cdn.example.id
  method      = "GET"
  domain      = "api.example.com"
  path        = "/v1/things"
  description = "Retrieve things"
  tag_ids     = [fastly_api_security_operation_tag.example.tag_id]
}
```

## Schema

### Required

- `name` (String) The name of the operation tag.
- `service_id` (String) Service ID the tag belongs to.

### Optional

- `description` (String) The description of the operation tag.

### Read-Only

- `created_at` (String) Created timestamp (when present).
- `id` (String) Alphanumeric string identifying the resource.
- `operation_count` (Number) Number of operations associated with this tag (when present).
- `tag_id` (String) The tag ID.
- `updated_at` (String) Updated timestamp (when present).

## Import

API Security operation tags can be imported using a composite ID of the form `service_id/tag_id`.

```shell
terraform import fastly_api_security_operation_tag.example SERVICE_ID/TAG_ID
```
