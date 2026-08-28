resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_threshold" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  action      = "block"
  dont_notify = false
  enabled     = true
  name        = "{{.THRESHOLD_NAME}}"
  signal      = "SQLI"
}
