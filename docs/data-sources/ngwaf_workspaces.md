---
page_title: "fastly_ngwaf_workspaces Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly Next-Gen WAF workspaces.
---

# fastly_ngwaf_workspaces (Data Source)

Use this data source to retrieve a list of [Fastly Next-Gen WAF workspaces](https://www.fastly.com/documentation/reference/api/ngwaf/workspaces/).

## Example Usage

```terraform
data "fastly_ngwaf_workspaces" "example" {}

output "fastly_ngwaf_workspaces_all" {
  value = data.fastly_ngwaf_workspaces.example.workspaces
}

output "fastly_ngwaf_workspaces_filtered" {
  # Example: get the ID of the workspace named "my_workspace"
  value = one([
    for workspace in data.fastly_ngwaf_workspaces.example.workspaces :
    workspace.id if workspace.name == "my_workspace"
  ])
}
```

## Schema

### Read-Only

- `id` (String) Terraform data source identifier.
- `workspaces` (Attributes Set) List of all Next-Gen WAF workspaces. (see [below for nested schema](#nestedatt--workspaces))

<a id="nestedatt--workspaces"></a>
### Nested Schema for `workspaces`

Read-Only:

- `id` (String) Identifier of the workspace.
- `name` (String) Name of the workspace.
