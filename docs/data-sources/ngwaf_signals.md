---
page_title: "fastly_ngwaf_signals Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve Fastly Next-Gen WAF custom signals defined at account scope.
---

# fastly_ngwaf_signals (Data Source)

Use this data source to retrieve Fastly Next-Gen WAF custom signals defined at
account scope.

Account-scoped NGWAF data sources omit `account` from their Terraform names.
To list custom signals from one workspace instead, use
`fastly_ngwaf_workspace_signals`.

The returned list is sorted by signal ID before it is stored in Terraform state,
so API response ordering does not cause state churn.

## Example Usage

```terraform
data "fastly_ngwaf_signals" "all" {}

output "ngwaf_signal_reference_ids" {
  value = [
    for signal in data.fastly_ngwaf_signals.all.signals :
    signal.reference_id
  ]
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `signals` (Attributes List) The list of custom signals defined at account scope. (see [below for nested schema](#nestedatt--signals))

<a id="nestedatt--signals"></a>
### Nested Schema for `signals`

Read-Only:

- `applies_to` (Set of String) The workspaces the signal applies to: a set of workspace IDs, or the single entry `*` for every workspace in the account.
- `description` (String) The description of the signal.
- `id` (String) The ID of the signal.
- `name` (String) The name of the signal.
- `reference_id` (String) The generated reference ID of the signal.
