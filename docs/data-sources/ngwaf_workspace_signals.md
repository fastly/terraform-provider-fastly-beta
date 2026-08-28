---
page_title: "fastly_ngwaf_workspace_signals Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve Fastly Next-Gen WAF custom signals scoped to a single workspace.
---

# fastly_ngwaf_workspace_signals (Data Source)

Use this data source to retrieve Fastly Next-Gen WAF custom signals scoped to a
single workspace.

## Example Usage

```terraform
data "fastly_ngwaf_workspace_signals" "example" {
  workspace_id = fastly_ngwaf_workspace.example.id
}

output "workspace_signals" {
  value = data.fastly_ngwaf_workspace_signals.example.signals
}
```

## Schema

### Required

- `workspace_id` (String) The ID of the workspace.

### Read-Only

- `id` (String) Terraform data source identifier.
- `signals` (Attributes List) The list of custom signals scoped to the workspace. (see [below for nested schema](#nestedatt--signals))

<a id="nestedatt--signals"></a>
### Nested Schema for `signals`

Read-Only:

- `description` (String) The description of the signal.
- `id` (String) The ID of the signal.
- `name` (String) The name of the signal.
- `reference_id` (String) The generated reference ID of the signal.
