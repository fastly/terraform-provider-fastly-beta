---
page_title: "fastly_ngwaf_rules Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve the list of Fastly Next-Gen WAF rules defined at account scope.
---

# fastly_ngwaf_rules (Data Source)

Use this data source to retrieve the list of Fastly Next-Gen WAF rules defined
at account scope, across every workspace they apply to.

Each entry reports its own `scope` and the workspaces it `applies_to`, and its
`type` says which resource manages it — `fastly_ngwaf_request_rule` or
`fastly_ngwaf_signal_rule`. To list the rules of a single workspace, including
the workspace-only `rate_limit` and `templated_signal` types, use the
`fastly_ngwaf_workspace_rules` data source instead.

## Example Usage

```terraform
data "fastly_ngwaf_rules" "example" {}

output "fastly_ngwaf_rules_all" {
  value = data.fastly_ngwaf_rules.example.rules
}

output "fastly_ngwaf_rules_enabled_only" {
  value = [
    for rule in data.fastly_ngwaf_rules.example.rules : rule.id
    if rule.enabled
  ]
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `rules` (Attributes List) The list of rules. (see [below for nested schema](#nestedatt--rules))

<a id="nestedatt--rules"></a>
### Nested Schema for `rules`

Read-Only:

- `applies_to` (Set of String) The workspaces the rule applies to: a set of workspace IDs, or the single entry `*` for every workspace in the account.
- `created_at` (String) The date and time in ISO 8601 format when the rule was created.
- `description` (String) The description of the rule.
- `enabled` (Boolean) Whether the rule is currently enabled.
- `id` (String) The ID of the rule.
- `type` (String) The type of the rule, either `request` or `signal` - the two types account scope supports.
- `updated_at` (String) The date and time in ISO 8601 format when the rule was last updated.
