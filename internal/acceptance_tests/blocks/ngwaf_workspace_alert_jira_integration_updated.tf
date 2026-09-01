resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_alert_jira_integration" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  description  = "{{.ALERT_DESCRIPTION}}"

  host         = "https://example-updated.atlassian.net"
  username     = "terraform-updated"
  project      = "TFUP"
  authentication = {
    key = "1234567890abcdef1234567890abcdef2"
  }
  issue_type   = "Bug"
}
