---
page_title: "fastly_api_security_operation Resource - fastly"
subcategory: ""
description: |-
  Manages an API Security operation for a Fastly service.
---

# fastly_api_security_operation (Resource)

Manages an API Security operation for a Fastly service. Operations represent API endpoints (method + domain + path) and can optionally be associated with operation tags.

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

resource "fastly_api_security_operation" "example" {
  service_id  = fastly_service_cdn.example.id
  method      = "GET"
  domain      = "api.example.com"
  path        = "/v1/things"
  description = "Retrieve things"
}
```

## Schema

### Required

- `domain` (String) Domain for the operation (exact match). Can be created, but not updated.
- `method` (String) HTTP method for the operation (e.g. GET, POST). Can be created, but not updated.
- `path` (String) Path for the operation (exact match). Can be created, but not updated.
- `service_id` (String) Service ID the operation belongs to. To import, use: <service_id>/<operation_id>.

### Optional

- `description` (String) A description of the operation.
- `tag_ids` (Set of String) Associated operation tag IDs.

### Read-Only

- `created_at` (String) Created timestamp (when present).
- `id` (String) Alphanumeric string identifying the resource. Format: `service_id/operation_id`.
- `last_seen_at` (String) Last seen timestamp (when present).
- `operation_id` (String) The operation ID.
- `rps` (Number) Observed requests per second (when present).
- `status` (String) Discovery status (when present).
- `updated_at` (String) Updated timestamp (when present).

## Import

API Security operations can be imported using a composite ID of the form `service_id/operation_id`.

```shell
terraform import fastly_api_security_operation.example SERVICE_ID/OPERATION_ID
```
