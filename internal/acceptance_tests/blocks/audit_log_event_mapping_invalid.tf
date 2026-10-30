resource "fastly_audit_log_event_mapping" "test" {
  name            = "{{.NAME}}"
  scope_type      = "{{.SCOPE_TYPE}}"
  scope_ids       = {{.SCOPE_IDS}}
  event_types     = ["address.create"]
  integration_ids = ["not-a-real-integration"]
}
