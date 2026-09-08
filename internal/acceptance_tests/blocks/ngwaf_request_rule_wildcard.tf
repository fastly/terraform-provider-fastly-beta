# A wildcard rule applies to every workspace in the account, so it is kept
# disabled: the point of the test is the applies_to round-trip, not enforcement.
resource "fastly_ngwaf_request_rule" "test" {
  applies_to  = ["*"]
  description = "{{.WORKSPACE_NAME}} wildcard round-trip"
  enabled     = false

  condition {
    field    = "path"
    operator = "equals"
    value    = "/{{.WORKSPACE_NAME}}"
  }

  action {
    type = "block"
  }
}
