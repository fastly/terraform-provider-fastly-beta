---
page_title: "fastly_bot_management_bots Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to list the predefined bots in a Bot Management (ContentGuard) workspace.
---

# fastly_bot_management_bots (Data Source)

Use this data source to list the predefined bots in a Bot Management
(ContentGuard) workspace, optionally limited to one category, for example to
find the `bot_id` to use with `fastly_bot_management_bot`.

## Example Usage

```terraform
data "fastly_bot_management_workspace" "example" {
  service_id = fastly_service_cdn_auto.example.id
}

data "fastly_bot_management_bots" "ai_crawler" {
  workspace_id = data.fastly_bot_management_workspace.example.id
  category_id  = "cat_ai_crawler" # omit to list bots in every category
}

output "ai_crawler_bot_ids" {
  value = [for b in data.fastly_bot_management_bots.ai_crawler.bots : b.bot_id]
}
```

## Schema

### Required

- `workspace_id` (String) The ID of the Bot Management workspace.

### Optional

- `category_id` (String) Only list the bots in this category. When omitted, bots in every category are listed.

### Read-Only

- `bots` (Attributes List) The bots in the Bot Management workspace. (see [below for nested schema](#nestedatt--bots))
- `id` (String) Terraform data source identifier.

<a id="nestedatt--bots"></a>
### Nested Schema for `bots`

Read-Only:

- `action` (String) The bot's action. One of `allow`, `block`, `challenge`, or `inherit` (follows the category's action).
- `bot_id` (String) The ID of the bot.
- `category_id` (String) The ID of the category the bot belongs to.
- `name` (String) The display name of the bot.
