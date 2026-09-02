resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for account rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_signal_rule" "test" {
  applies_to  = [fastly_ngwaf_workspace.test.id]
  description = "Exclude a false-positive signal account-wide"
  enabled     = true

  condition {
    field    = "path"
    operator = "like"
    value    = "/contact-form"
  }

  action {
    signal = "XSS"
  }
}
