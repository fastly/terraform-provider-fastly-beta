---
page_title: "fastly_ngwaf_workspace_alert_microsoft_teams_integrations Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve Microsoft Teams alert integrations scoped to a single Fastly Next-Gen WAF workspace.
---

# fastly_ngwaf_workspace_alert_microsoft_teams_integrations (Data Source)

Use this data source to retrieve `microsoftteams` alert integrations scoped to a single Fastly Next-Gen WAF workspace.

## Example Usage

```terraform
data "fastly_ngwaf_workspace_alert_microsoft_teams_integrations" "example" {
  workspace_id = fastly_ngwaf_workspace.example.id
}

output "ngwaf_workspace_alert_microsoft_teams_integrations" {
  value = data.fastly_ngwaf_workspace_alert_microsoft_teams_integrations.example.alerts
}
```

## Schema

### Required

- `workspace_id` (String) The ID of the workspace.

### Read-Only

- `alerts` (Attributes List) The list of microsoftteams alert integrations scoped to the workspace. (see [below for nested schema](#nestedatt--alerts))
- `id` (String) Terraform data source identifier.

<a id="nestedatt--alerts"></a>
### Nested Schema for `alerts`

Read-Only:

- `description` (String) The description of the alert integration.
- `id` (String) The ID of the alert integration.
