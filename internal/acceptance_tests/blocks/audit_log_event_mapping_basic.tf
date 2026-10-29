resource "fastly_integration" "test" {
  name        = "{{.INTEGRATION_NAME}}"
  description = "created by acceptance test"
  type        = "webhook"

  authentication = {
    webhook = "https://example.com/hooks/audit"
  }
}

resource "fastly_audit_log_event_mapping" "test" {
  name            = "{{.NAME}}"
  description     = "{{.DESCRIPTION}}"
  scope_type      = "{{.SCOPE_TYPE}}"
  scope_ids       = {{.SCOPE_IDS}}
  event_types     = {{.EVENT_TYPES}}
  integration_ids = [fastly_integration.test.id]
}
