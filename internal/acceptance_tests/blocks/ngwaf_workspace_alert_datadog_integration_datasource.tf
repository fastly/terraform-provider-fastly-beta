resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_alert_datadog_integration" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  description  = "{{.ALERT_DESCRIPTION}}"

  authentication = {
    key = "1234567890abcdef1234567890abcdef"
  }
  site         = "us1"
}

data "fastly_ngwaf_workspace_alert_datadog_integrations" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  depends_on = [fastly_ngwaf_workspace_alert_datadog_integration.test]
}
