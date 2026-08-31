resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_alert_mailing_list_integration" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  description  = "{{.ALERT_DESCRIPTION}}"

  address      = "alerts@example.com"
}

data "fastly_ngwaf_workspace_alert_mailing_list_integrations" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  depends_on = [fastly_ngwaf_workspace_alert_mailing_list_integration.test]
}
