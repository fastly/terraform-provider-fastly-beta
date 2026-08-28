resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_signal" "signal_1" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.SIGNAL_NAME_1}}"
  description  = "First test signal"
}

resource "fastly_ngwaf_workspace_signal" "signal_2" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.SIGNAL_NAME_2}}"
  description  = "Second test signal"
}

data "fastly_ngwaf_workspace_signals" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  depends_on = [
    fastly_ngwaf_workspace_signal.signal_1,
    fastly_ngwaf_workspace_signal.signal_2,
  ]
}
