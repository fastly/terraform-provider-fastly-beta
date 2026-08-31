resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_alert_microsoft_teams_integration" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  description  = "{{.ALERT_DESCRIPTION}}"

  webhook      = "https://example.com/webhooks/my-service"
}
