resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for rules data source"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_rule" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  type         = "request"
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

data "fastly_ngwaf_workspace_rules" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  depends_on = [fastly_ngwaf_workspace_rule.test]
}
