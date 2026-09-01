---
page_title: "fastly_ngwaf_workspace_redactions Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve Fastly Next-Gen WAF field redactions scoped to a single workspace.
---

# fastly_ngwaf_workspace_redactions (Data Source)

Use this data source to retrieve Fastly Next-Gen WAF field redactions scoped
to a single workspace.

## Example Usage

```terraform
data "fastly_ngwaf_workspace_redactions" "example" {
  workspace_id = fastly_ngwaf_workspace.example.id
}

output "workspace_redactions" {
  value = data.fastly_ngwaf_workspace_redactions.example.redactions
}
```

## Schema

### Required

- `workspace_id` (String) The ID of the workspace.

### Read-Only

- `id` (String) Terraform data source identifier.
- `redactions` (Attributes List) The list of field redactions scoped to the workspace. (see [below for nested schema](#nestedatt--redactions))

<a id="nestedatt--redactions"></a>
### Nested Schema for `redactions`

Read-Only:

- `field` (String) The name of the field that is being redacted.
- `id` (String) The ID of the redaction.
- `type` (String) The type of field being redacted. One of `request_parameter`, `request_header`, or `response_header`.
