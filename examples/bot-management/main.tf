terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly-beta"
    }
  }
}

provider "fastly" {
  # API token set via FASTLY_API_TOKEN environment variable
}

resource "fastly_service_cdn_auto" "example" {
  name = "bot-management-example"

  domain {
    name = "www.example.fastly.com"
  }

  backend {
    name    = "origin"
    address = "origin.example.com"
    port    = 443
    use_ssl = true
  }

  force_destroy = true
}

# Enabling ContentGuard creates the service's Bot Management workspace.
# Apply this before the rest of the configuration; see README.md.
resource "fastly_service_product_bot_management" "example" {
  service_id   = fastly_service_cdn_auto.example.id
  contentguard = "on"
}

data "fastly_bot_management_workspace" "example" {
  service_id = fastly_service_cdn_auto.example.id
}

resource "fastly_bot_management_workspace" "example" {
  workspace_id    = data.fastly_bot_management_workspace.example.id
  protection_mode = "block"
}

# Block all AI crawlers by default, including ones added later.
resource "fastly_bot_management_category" "ai_crawler" {
  workspace_id = data.fastly_bot_management_workspace.example.id
  category_id  = "cat_ai_crawler"
  action       = "block"
}

# Allow GPTBot, overriding the category action. Destroying this resource leaves
# GPTBot's action unchanged; set `action = "inherit"` first to return it to the
# category action.
resource "fastly_bot_management_bot" "gptbot" {
  workspace_id = data.fastly_bot_management_workspace.example.id
  category_id  = fastly_bot_management_category.ai_crawler.category_id
  bot_id       = "bot_gptbot"
  action       = "allow"
}

# Discover the categories and bots available in the workspace.
data "fastly_bot_management_categories" "all" {
  workspace_id = data.fastly_bot_management_workspace.example.id
}

data "fastly_bot_management_bots" "ai_crawler" {
  workspace_id = data.fastly_bot_management_workspace.example.id
  category_id  = "cat_ai_crawler"
}

output "category_ids" {
  value = [for c in data.fastly_bot_management_categories.all.categories : c.category_id]
}

output "ai_crawler_bot_ids" {
  value = [for b in data.fastly_bot_management_bots.ai_crawler.bots : b.bot_id]
}

# Clear the bot overrides in a given category, so every bot uses its inherited
# category default, for example with
# `terraform apply -invoke=action.fastly_bot_management_category_overrides_clear.ai_crawler`.
action "fastly_bot_management_category_overrides_clear" "ai_crawler" {
  config {
    workspace_id = data.fastly_bot_management_workspace.example.id
    category_id  = fastly_bot_management_category.ai_crawler.category_id
  }
}
