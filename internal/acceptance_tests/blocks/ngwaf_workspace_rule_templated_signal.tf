resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_rule" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  type         = "templated_signal"
  description  = ""
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  action {
    type   = "templated_signal"
    signal = "LOGINATTEMPT"
  }
}
