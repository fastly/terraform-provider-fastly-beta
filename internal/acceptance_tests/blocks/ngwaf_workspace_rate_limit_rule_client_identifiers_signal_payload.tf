resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for a rate_limit rule"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_signal" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  name         = "{{.SIGNAL_NAME}}"
  description  = "Signal used by the rate_limit rule under test"
}

resource "fastly_ngwaf_workspace_rate_limit_rule" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  description  = "Rate limit by signal payload"
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  rate_limit {
    duration  = 300
    interval  = 60
    signal    = fastly_ngwaf_workspace_signal.test.reference_id
    threshold = 100

    client_identifiers {
      type   = "signal_payload"
      signal = fastly_ngwaf_workspace_signal.test.reference_id
    }
  }

  action {
    type   = "log_request"
    signal = fastly_ngwaf_workspace_signal.test.reference_id
  }
}
