---
page_title: "fastly_bot_management_bot Resource - fastly"
subcategory: ""
description: |-
  Manages the action of a predefined bot in a Bot Management (ContentGuard) workspace.
---

# fastly_bot_management_bot (Resource)

Manages the action of a predefined bot in a Bot Management (ContentGuard)
workspace. Set `action` to `allow`, `block`, or `challenge` to override the
action of the category the bot belongs to, or to `inherit` to keep the bot on
its inherited category default.

Bots are predefined and can't be created or deleted. This resource doesn't
create or delete bots; it sets the action of an existing bot. Destroying the
resource sets the bot's action back to `inherit`.

Setting `action = "inherit"` keeps the bot on its category's action. If the
bot's action is changed outside Terraform, the next `apply` sets it back to
`inherit`.

~> **Note:** The `fastly_bot_management_category_reset_bots` action can be used
to reset every bot in a category to `inherit`, including bots managed by this
resource. The next `apply` sets bots configured with another action back to
that action.

Use the `fastly_bot_management_bots` data source to list the bot IDs available
in a workspace or category.

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

resource "fastly_bot_management_bot" "gptbot" {
  workspace_id = data.fastly_bot_management_workspace.example.id
  category_id  = fastly_bot_management_category.ai_crawler.category_id
  bot_id       = "bot_gptbot"
  action       = "allow"
}
```

## Schema

### Required

- `action` (String) The action applied to requests from the bot. One of `allow`, `block`, or `challenge` to override the category's action, or `inherit` to use the category's action. Destroying this resource sets the action to `inherit`.
- `bot_id` (String) The ID of the bot, such as `bot_gptbot`. Use the `fastly_bot_management_bots` data source to list the available IDs.
- `category_id` (String) The ID of the category the bot belongs to, such as `cat_ai_crawler`.
- `workspace_id` (String) The ID of the Bot Management workspace the bot belongs to.

### Read-Only

- `id` (String) The resource ID, in the form `workspace_id/category_id/bot_id`.
- `name` (String) The display name of the bot.

## Import

Bot Management bots are imported using the workspace ID, category ID and bot
ID, separated by `/`.

```shell
terraform import fastly_bot_management_bot.gptbot WORKSPACE_ID/CATEGORY_ID/BOT_ID
```
