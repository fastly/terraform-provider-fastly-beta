resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for account rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace" "second" {
  name        = "{{.WORKSPACE_NAME}}-second"
  description = "Second test NGWAF Workspace for account rules"
  mode        = "log"

  attack_signal_thresholds {}
}

# applies_to gains a second workspace, and the rule is expected to update in
# place: unlike a workspace rule's workspace_id, it is a request body field
# rather than a path segment.
resource "fastly_ngwaf_request_rule" "test" {
  applies_to = [
    fastly_ngwaf_workspace.test.id,
    fastly_ngwaf_workspace.second.id,
  ]
  description     = "Allow a specific path account-wide"
  enabled         = false
  group_operator  = "any"
  request_logging = "sampled"

  condition {
    field    = "path"
    operator = "equals"
    value    = "/health"
  }

  action {
    type = "allow"
  }
}
