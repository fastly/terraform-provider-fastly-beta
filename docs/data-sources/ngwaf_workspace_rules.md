---
page_title: "fastly_ngwaf_workspace_rules Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly Next-Gen WAF rules scoped to a single workspace.
---

# fastly_ngwaf_workspace_rules (Data Source)

Use this data source to retrieve a list of Fastly Next-Gen WAF rules scoped to a single workspace.

Rules of every type are returned; `type` on each entry says which of the
`fastly_ngwaf_workspace_*_rule` resources manages it.

## Example Usage

```terraform
data "fastly_ngwaf_workspace_rules" "example" {
  workspace_id = fastly_ngwaf_workspace.example.id
}

output "fastly_ngwaf_workspace_rules_all" {
  value = data.fastly_ngwaf_workspace_rules.example.rules
}
```

## Schema

### Required

- `workspace_id` (String) The ID of the workspace.

### Read-Only

- `id` (String) Terraform data source identifier.
- `rules` (Attributes List) The list of rules. (see [below for nested schema](#nestedatt--rules))

<a id="nestedatt--rules"></a>
### Nested Schema for `rules`

Read-Only:

- `created_at` (String) The date and time in ISO 8601 format when the rule was created.
- `description` (String) The description of the rule.
- `enabled` (Boolean) Whether the rule is currently enabled.
- `id` (String) The ID of the rule.
- `type` (String) The type of the rule.
- `updated_at` (String) The date and time in ISO 8601 format when the rule was last updated.
