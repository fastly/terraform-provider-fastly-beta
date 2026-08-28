---
page_title: "fastly_ngwaf_workspace_lists Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly Next-Gen WAF lists scoped to a single workspace.
---

# fastly_ngwaf_workspace_lists (Data Source)

Use this data source to retrieve a list of Fastly Next-Gen WAF lists scoped to a
single workspace.

Lists of every type are returned; `type` on each entry says which of the
`fastly_ngwaf_workspace_*_list` resources manages it.

## Example Usage

```terraform
data "fastly_ngwaf_workspace_lists" "example" {
  workspace_id = fastly_ngwaf_workspace.example.id
}

output "fastly_ngwaf_workspace_lists_all" {
  value = data.fastly_ngwaf_workspace_lists.example.lists
}
```

## Schema

### Required

- `workspace_id` (String) The ID of the workspace.

### Read-Only

- `id` (String) Terraform data source identifier.
- `lists` (Attributes List) The list of workspace-scoped NGWAF lists. (see [below for nested schema](#nestedatt--lists))

<a id="nestedatt--lists"></a>
### Nested Schema for `lists`

Read-Only:

- `created_at` (String) The date and time in ISO 8601 format when the list was created.
- `description` (String) The description of the list.
- `id` (String) The ID of the list.
- `name` (String) The name of the list.
- `reference_id` (String) The reference ID of the list.
- `type` (String) The type of the list.
- `updated_at` (String) The date and time in ISO 8601 format when the list was last updated.
