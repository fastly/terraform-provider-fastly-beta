resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_alert_datadog_integration" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  description  = "{{.ALERT_DESCRIPTION}}"

  key          = "1234567890abcdef1234567890abcdef2"
  site         = "eu1"
}
