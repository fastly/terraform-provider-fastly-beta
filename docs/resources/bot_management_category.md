---
page_title: "fastly_bot_management_category Resource - fastly"
subcategory: ""
description: |-
  Manages the action of a predefined bot category in a Bot Management (ContentGuard) workspace.
---

# fastly_bot_management_category (Resource)

Manages the action of a predefined bot category in a Bot Management
(ContentGuard) workspace.

The category's action applies to every bot in the category that is set to
`inherit`, including bots added to the category later. Bots with
their own action, set with `fastly_bot_management_bot`, keep it. To clear
every bot override in a category at once, use the
`fastly_bot_management_category_overrides_clear` action.

Categories are predefined and can't be created or deleted. This resource
doesn't create or delete categories; it sets the action of an existing
category. Destroying the resource only removes it from Terraform state, and the
category's action is left unchanged.

Use the `fastly_bot_management_categories` data source to list the category
IDs available in a workspace.

## Example Usage

```terraform
data "fastly_bot_management_workspace" "example" {
  service_id = fastly_service_cdn_auto.example.id
}

resource "fastly_bot_management_category" "ai_crawler" {
  workspace_id = data.fastly_bot_management_workspace.example.id
  category_id  = "cat_ai_crawler"
  action       = "block"
}
```

## Schema

### Required

- `action` (String) The action applied to every bot in the category that is set to `inherit`, including bots added to the category later. One of `allow`, `block`, or `challenge`.
- `category_id` (String) The ID of the bot category, such as `cat_ai_crawler`. Use the `fastly_bot_management_categories` data source to list the available IDs.
- `workspace_id` (String) The ID of the Bot Management workspace the category belongs to.

### Read-Only

- `description` (String) A description of the bots the category contains.
- `id` (String) The resource ID, in the form `workspace_id/category_id`.
- `name` (String) The display name of the category.

## Import

Bot Management categories are imported using the workspace ID and category
ID, separated by `/`.

```shell
terraform import fastly_bot_management_category.ai_crawler WORKSPACE_ID/CATEGORY_ID
```
