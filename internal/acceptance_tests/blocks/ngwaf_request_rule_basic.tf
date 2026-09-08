resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for account rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace" "second" {
  name        = "{{.WORKSPACE_NAME}}-second"
  description = "Second test NGWAF Workspace for account rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_request_rule" "test" {
  applies_to  = [fastly_ngwaf_workspace.test.id]
  description = "Block a specific IP account-wide"
  enabled     = true

  condition {
    field    = "ip"
    operator = "equals"
    value    = "127.0.0.1"
  }

  action {
    type = "block"
  }
}
