---
page_title: "fastly_ngwaf_workspace_virtual_patches Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to retrieve a list of Fastly Next-Gen WAF virtual patches for a workspace.
---

# fastly_ngwaf_workspace_virtual_patches (Data Source)

Use this data source to retrieve a list of Fastly Next-Gen WAF virtual patches
for a workspace.

## Example Usage

```terraform
data "fastly_ngwaf_workspace_virtual_patches" "example" {
  workspace_id = fastly_ngwaf_workspace.example.id
}

output "ngwaf_workspace_virtual_patches" {
  value = data.fastly_ngwaf_workspace_virtual_patches.example.virtual_patches
}
```

## Schema

### Required

- `workspace_id` (String) The ID of the workspace.

### Read-Only

- `id` (String) Terraform data source identifier.
- `virtual_patches` (Attributes List) List of all virtual patches for the workspace. (see [below for nested schema](#nestedatt--virtual_patches))

<a id="nestedatt--virtual_patches"></a>
### Nested Schema for `virtual_patches`

Read-Only:

- `description` (String) Description of the virtual patch.
- `enabled` (Boolean) Whether the virtual patch is enabled.
- `id` (String) The ID of the virtual patch.
- `mode` (String) Action to take when a signal for virtual patch is detected.
