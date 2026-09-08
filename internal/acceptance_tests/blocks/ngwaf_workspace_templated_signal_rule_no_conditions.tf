resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_templated_signal_rule" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  enabled      = true

  action {
    signal = "LOGINATTEMPT"
  }
}
