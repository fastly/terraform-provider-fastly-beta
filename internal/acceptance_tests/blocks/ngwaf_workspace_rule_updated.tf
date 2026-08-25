resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_rule" "test" {
  workspace_id    = fastly_ngwaf_workspace.test.id
  type            = "request"
  description     = "Allow a specific path"
  enabled         = false
  group_operator  = "any"
  request_logging = "sampled"

  condition {
    field    = "path"
    operator = "contains"
    value    = "/admin"
  }

  action {
    type = "allow"
  }
}
