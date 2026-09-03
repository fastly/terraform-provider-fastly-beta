---
page_title: "fastly_ngwaf_lists Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve Fastly Next-Gen WAF lists defined at account scope.
---

# fastly_ngwaf_lists (Data Source)

Use this data source to retrieve Fastly Next-Gen WAF lists defined at account
scope.

Lists of every type are returned; `type` on each entry identifies which
type-specific resource manages it:

- `fastly_ngwaf_ip_list`
- `fastly_ngwaf_string_list`
- `fastly_ngwaf_wildcard_list`
- `fastly_ngwaf_country_list`
- `fastly_ngwaf_signal_list`

Account-scoped NGWAF data sources omit `account` from their Terraform names.
To retrieve lists from one workspace instead, use
`fastly_ngwaf_workspace_lists`.

The returned list is sorted by list ID before it is stored in Terraform state,
so API response ordering does not cause state churn.

## Example Usage

```terraform
data "fastly_ngwaf_lists" "example" {}

output "fastly_ngwaf_lists_all" {
  value = data.fastly_ngwaf_lists.example.lists
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `lists` (Attributes List) The list of account-scoped NGWAF lists. (see [below for nested schema](#nestedatt--lists))

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
