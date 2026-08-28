resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_signal_list" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.LIST_NAME}}"
  description  = "{{.LIST_DESCRIPTION}}"

  entries = {{.LIST_ENTRIES}}
}
