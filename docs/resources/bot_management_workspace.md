---
page_title: "fastly_bot_management_workspace Resource - fastly"
subcategory: ""
description: |-
  Manages the settings of an existing Fastly Bot Management (ContentGuard) workspace.
---

# fastly_bot_management_workspace (Resource)

Manages the settings of an existing Fastly Bot Management (ContentGuard)
workspace: its protection mode, name and description.

~> **Note:** A Bot Management workspace isn't a Next-Gen WAF workspace. Its ID
can't be used with `fastly_ngwaf_*` resources, and `fastly_ngwaf_workspace` IDs
can't be used here.

A Bot Management workspace is created when ContentGuard is enabled on a
service, and there's no API to create or delete one. This resource doesn't
create or delete the workspace; it manages the settings of an existing one.
Destroying the resource only removes it from Terraform state, and the
workspace's settings are left unchanged.

!> **Warning:** Do not enable ContentGuard with
`fastly_service_product_bot_management` and manage its Bot Management workspace
in the same `apply`. Otherwise, the `fastly_bot_management_workspace` data
source can look up the workspace before it exists, and the apply fails with a
"Bot Management workspace not found" error. Enable ContentGuard first, then add
the data source and the Bot Management resources in a later `apply`.

`protection_mode` gates all enforcement in the workspace:

* `off`: no enforcement, regardless of category and bot actions.
* `log`: bots are detected and logged, but no requests are blocked or challenged.
* `block`: category and bot actions are fully enforced.

## Example Usage

```terraform
data "fastly_bot_management_workspace" "example" {
  service_id = fastly_service_cdn_auto.example.id
}

resource "fastly_bot_management_workspace" "example" {
  workspace_id    = data.fastly_bot_management_workspace.example.id
  protection_mode = "block"
}
```

## Schema

### Required

- `protection_mode` (String) The protection mode of the Bot Management workspace, which gates all enforcement. One of `off` (no enforcement), `log` (bots are detected and logged, but no requests are blocked or challenged), or `block` (category and bot actions are enforced).
- `workspace_id` (String) The ID of the Bot Management workspace to manage. Use the `fastly_bot_management_workspace` data source to look it up from a service ID.

### Optional

- `description` (String) A description of the Bot Management workspace. Must be at most 1000 characters. When omitted, the current description is kept.
- `name` (String) The display name of the Bot Management workspace. Must be between 1 and 255 characters. When omitted, the current name is kept.

### Read-Only

- `id` (String) The ID of the Bot Management workspace. Same as `workspace_id`.

## Import

Bot Management workspaces are imported using the workspace ID.

```shell
terraform import fastly_bot_management_workspace.example WORKSPACE_ID
```
