---
page_title: "fastly_ngwaf_thresholds Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly Next-Gen WAF thresholds for a workspace.
---

# fastly_ngwaf_thresholds (Data Source)

Use this data source to retrieve a list of Fastly Next-Gen WAF thresholds for a workspace.

## Example Usage

```terraform
data "fastly_ngwaf_thresholds" "example" {
  workspace_id = fastly_ngwaf_workspace.example.id
}

output "ngwaf_thresholds" {
  value = data.fastly_ngwaf_thresholds.example.thresholds
}
```

## Schema

### Required

- `workspace_id` (String) The ID of the workspace.

### Read-Only

- `id` (String) Terraform data source identifier.
- `thresholds` (Attributes Set) List of all thresholds for the workspace. (see [below for nested schema](#nestedatt--thresholds))

<a id="nestedatt--thresholds"></a>
### Nested Schema for `thresholds`

Read-Only:

- `action` (String) Action to take when threshold is exceeded.
- `dont_notify` (Boolean) Whether to silence notifications when action is taken.
- `duration` (Number) Duration the action is in place, in seconds.
- `enabled` (Boolean) Whether this threshold is active.
- `id` (String) The ID of the threshold.
- `interval` (Number) Threshold interval in seconds.
- `limit` (Number) Threshold limit.
- `name` (String) The name of the threshold.
- `signal` (String) The name of the signal this threshold is acting on.
