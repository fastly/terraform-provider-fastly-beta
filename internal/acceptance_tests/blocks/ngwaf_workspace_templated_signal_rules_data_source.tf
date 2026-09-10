resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for templated signal rules data source"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_request_rule" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  description  = "Block a specific IP"
  enabled      = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "127.0.0.1"
  }

  action {
    type = "block"
  }
}

resource "fastly_ngwaf_workspace_templated_signal_rule" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  action {
    signal = "LOGINATTEMPT"
  }
}

data "fastly_ngwaf_workspace_rules" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  depends_on = [
    fastly_ngwaf_workspace_request_rule.test,
    fastly_ngwaf_workspace_templated_signal_rule.test,
  ]
}

data "fastly_ngwaf_workspace_templated_signal_rules" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  depends_on = [fastly_ngwaf_workspace_templated_signal_rule.test]
}
