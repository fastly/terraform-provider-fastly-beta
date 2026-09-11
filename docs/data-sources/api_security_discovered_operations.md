---
page_title: "fastly_api_security_discovered_operations Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to list API Security discovered operations for a service, optionally filtered by domain, method, path, or status.
---

# fastly_api_security_discovered_operations (Data Source)

Use this data source to list API Security discovered operations for a service, optionally filtered by domain, method, path, or status.

## Example Usage

```terraform
data "fastly_api_security_discovered_operations" "example" {
  service_id = fastly_service_cdn.example.id
  status     = "DISCOVERED"
}

output "fastly_api_security_discovered_operations_all" {
  value = data.fastly_api_security_discovered_operations.example.operations
}
```

## Schema

### Required

- `service_id` (String) Service ID.

### Optional

- `domain` (Set of String) Filter by one or more fully-qualified domains (exact match).
- `method` (Set of String) Filter by one or more HTTP methods.
- `path` (String) Filter by path (exact match).
- `status` (String) Filter discovered operations by status. Accepted values are `DISCOVERED`, `SAVED`, and `IGNORED`.

### Read-Only

- `id` (String) Terraform data source identifier.
- `operations` (Attributes List) Discovered operations. (see [below for nested schema](#nestedatt--operations))
- `total` (Number) Total number of matching results, as returned by the API.

<a id="nestedatt--operations"></a>
### Nested Schema for `operations`

Read-Only:

- `domain` (String) Discovered operation domain.
- `id` (String) Discovered operation ID.
- `last_seen_at` (String) Last seen timestamp (when present).
- `method` (String) Discovered operation HTTP method.
- `path` (String) Discovered operation path.
- `rps` (Number) Observed requests per second (when present).
- `status` (String) Discovered operation status (when present).
- `updated_at` (String) Updated timestamp (when present).
