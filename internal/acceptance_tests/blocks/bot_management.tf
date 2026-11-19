data "fastly_bot_management_workspace" "by_service" {
  service_id = fastly_service_cdn_auto.test.id
}

data "fastly_bot_management_workspace" "by_id" {
  workspace_id = data.fastly_bot_management_workspace.by_service.id
}

resource "fastly_bot_management_workspace" "test" {
  workspace_id    = data.fastly_bot_management_workspace.by_service.id
  name            = "{{.WORKSPACE_NAME}}"
  description     = "{{.WORKSPACE_DESCRIPTION}}"
  protection_mode = "{{.PROTECTION_MODE}}"
}

resource "fastly_bot_management_category" "test" {
  workspace_id = data.fastly_bot_management_workspace.by_service.id
  category_id  = "{{.CATEGORY_ID}}"
  action       = "{{.CATEGORY_ACTION}}"
}
{{if .BOT_ACTION}}
resource "fastly_bot_management_bot" "test" {
  workspace_id = data.fastly_bot_management_workspace.by_service.id
  category_id  = fastly_bot_management_category.test.category_id
  bot_id       = "{{.BOT_ID}}"
  action       = "{{.BOT_ACTION}}"
}
{{end}}
data "fastly_bot_management_categories" "test" {
  workspace_id = data.fastly_bot_management_workspace.by_service.id
}

data "fastly_bot_management_bots" "test" {
  workspace_id = data.fastly_bot_management_workspace.by_service.id
  category_id  = "{{.CATEGORY_ID}}"
}
