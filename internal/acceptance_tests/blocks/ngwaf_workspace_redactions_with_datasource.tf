resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_redaction" "redaction_1" {
  workspace_id = fastly_ngwaf_workspace.test.id
  field        = "{{.REDACTION_FIELD_1}}"
  type         = "request_parameter"
}

resource "fastly_ngwaf_workspace_redaction" "redaction_2" {
  workspace_id = fastly_ngwaf_workspace.test.id
  field        = "{{.REDACTION_FIELD_2}}"
  type         = "request_header"
}

data "fastly_ngwaf_workspace_redactions" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  depends_on = [
    fastly_ngwaf_workspace_redaction.redaction_1,
    fastly_ngwaf_workspace_redaction.redaction_2,
  ]
}
