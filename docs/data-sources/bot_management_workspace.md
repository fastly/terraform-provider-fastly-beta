---
page_title: "fastly_bot_management_workspace Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to look up a Fastly Bot Management (ContentGuard) workspace.
---

# fastly_bot_management_workspace (Data Source)

Use this data source to look up a Fastly Bot Management (ContentGuard)
workspace, either by the ID of a service attached to it or by its workspace ID.
Exactly one of `service_id` or `workspace_id` must be set.

~> **Note:** A Bot Management workspace isn't a Next-Gen WAF workspace. Its ID
can't be used with `fastly_ngwaf_*` resources.

A Bot Management workspace is created when ContentGuard is enabled on a
service.

!> **Warning:** Do not enable ContentGuard with
`fastly_service_product_bot_management` and look up its Bot Management
workspace in the same `apply`. Otherwise, this data source can look up the
workspace before it exists, and the apply fails with a "Bot Management
workspace not found" error. Enable ContentGuard first, then add this data
source in a later `apply`.

## Example Usage

```terraform
data "fastly_bot_management_workspace" "by_service" {
  service_id = fastly_service_cdn_auto.example.id
}

data "fastly_bot_management_workspace" "by_id" {
  workspace_id = "WORKSPACE_ID"
}
```

## Schema

### Optional

- `service_id` (String) The ID of a service attached to the Bot Management workspace. Conflicts with `workspace_id`.
- `workspace_id` (String) The ID of the Bot Management workspace. Conflicts with `service_id`.

### Read-Only

- `created_at` (String) The date and time the Bot Management workspace was created, in RFC 3339 format.
- `description` (String) A description of the Bot Management workspace.
- `id` (String) The ID of the Bot Management workspace.
- `name` (String) The display name of the Bot Management workspace.
- `protection_mode` (String) The protection mode of the Bot Management workspace. One of `off`, `log`, or `block`.
- `services` (List of String) The IDs of the services attached to the Bot Management workspace.
- `updated_at` (String) The date and time the Bot Management workspace was last updated, in RFC 3339 format.
