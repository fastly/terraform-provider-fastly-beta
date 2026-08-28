resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_rate_limit_rule" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  description  = "Rate limit rule with two non-ip client identifiers"
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  rate_limit {
    duration  = 300
    interval  = 60
    signal    = "site.tf-test-signal"
    threshold = 100

    client_identifiers {
      type = "request_header"
      name = "X-Forwarded-For"
    }

    client_identifiers {
      type = "request_cookie"
      name = "session_id"
    }
  }

  action {
    type   = "log_request"
    signal = "site.tf-test-signal"
  }
}
