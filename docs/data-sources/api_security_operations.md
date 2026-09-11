---
page_title: "fastly_api_security_operations Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to list API Security operations for a service, optionally filtered by domain, method, path, or tag.
---

# fastly_api_security_operations (Data Source)

Use this data source to list API Security operations for a service, optionally filtered by domain, method, path, or tag.

## Example Usage

```terraform
data "fastly_api_security_operations" "example" {
  service_id = fastly_service_cdn.example.id
  method     = ["GET"]
}

output "fastly_api_security_operations_all" {
  value = data.fastly_api_security_operations.example.operations
}
```

## Schema

### Required

- `service_id` (String) Service ID.

### Optional

- `domain` (Set of String) Filter by one or more domains (exact match).
- `method` (Set of String) Filter by one or more HTTP methods.
- `path` (String) Filter by path (exact match).
- `tag_id` (String) Filter by tag ID.

### Read-Only

- `id` (String) Terraform data source identifier.
- `operations` (Attributes List) Matching API Security operations. (see [below for nested schema](#nestedatt--operations))
- `total` (Number) Total number of matching results, as returned by the API.

<a id="nestedatt--operations"></a>
### Nested Schema for `operations`

Read-Only:

- `created_at` (String) Created timestamp (when present).
- `description` (String) Operation description (when present).
- `domain` (String) Operation domain.
- `id` (String) Operation ID.
- `last_seen_at` (String) Last seen timestamp (when present).
- `method` (String) Operation HTTP method.
- `path` (String) Operation path.
- `rps` (Number) Observed requests per second (when present).
- `status` (String) Discovery status (when present). One of `DISCOVERED`, `SAVED`, or `IGNORED`.
- `tag_ids` (Set of String) Associated operation tag IDs.
- `updated_at` (String) Updated timestamp (when present).
