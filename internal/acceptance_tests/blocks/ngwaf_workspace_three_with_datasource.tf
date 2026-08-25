resource "fastly_ngwaf_workspace" "workspace_1" {
  name        = "{{.WORKSPACE_NAME_1}}"
  description = "Test NGWAF Workspace"
  mode        = "off"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace" "workspace_2" {
  name        = "{{.WORKSPACE_NAME_2}}"
  description = "Test NGWAF Workspace"
  mode        = "off"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace" "workspace_3" {
  name        = "{{.WORKSPACE_NAME_3}}"
  description = "Test NGWAF Workspace"
  mode        = "off"

  attack_signal_thresholds {}
}

data "fastly_ngwaf_workspaces" "example" {
  depends_on = [
    fastly_ngwaf_workspace.workspace_1,
    fastly_ngwaf_workspace.workspace_2,
    fastly_ngwaf_workspace.workspace_3,
  ]
}
