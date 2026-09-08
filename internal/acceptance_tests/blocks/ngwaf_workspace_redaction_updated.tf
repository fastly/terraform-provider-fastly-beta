resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_redaction" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  field        = "{{.REDACTION_FIELD_UPDATED}}"
  type         = "request_header"
}
