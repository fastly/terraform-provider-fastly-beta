---
page_title: "fastly_api_security_operation_tags Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to list API Security operation tags for a service.
---

# fastly_api_security_operation_tags (Data Source)

Use this data source to list API Security operation tags for a service.

## Example Usage

```terraform
data "fastly_api_security_operation_tags" "example" {
  service_id = fastly_service_cdn.example.id
}

output "fastly_api_security_operation_tags_all" {
  value = data.fastly_api_security_operation_tags.example.tags
}
```

## Schema

### Required

- `service_id` (String) Service ID.

### Read-Only

- `id` (String) Terraform data source identifier.
- `tags` (Attributes List) Operation tags. (see [below for nested schema](#nestedatt--tags))
- `total` (Number) Total number of matching results, as returned by the API.

<a id="nestedatt--tags"></a>
### Nested Schema for `tags`

Read-Only:

- `created_at` (String) Created timestamp (when present).
- `description` (String) Tag description (when present).
- `id` (String) Tag ID.
- `name` (String) Tag name.
- `operation_count` (Number) Number of operations associated with this tag (when present).
- `updated_at` (String) Updated timestamp (when present).
