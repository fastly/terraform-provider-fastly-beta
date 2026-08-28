resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_signal" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.SIGNAL_NAME}}"
  description  = "{{.SIGNAL_DESCRIPTION}}"
}
