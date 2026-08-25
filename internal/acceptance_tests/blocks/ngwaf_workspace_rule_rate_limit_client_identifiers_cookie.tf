resource "fastly_ngwaf_workspace_rule" "test" {
  workspace_id = "{{.WORKSPACE_ID}}"
  type         = "rate_limit"
  description  = "Rate limit by IP"
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/login"
  }

  rate_limit {
    duration  = 300
    interval  = 60
    signal    = "{{.SIGNAL_ID}}"
    threshold = 100

    client_identifiers {
      type = "ip"
    }

    client_identifiers {
      type = "request_cookie"
      name = "session_id"
    }
  }

  action {
    type   = "log_request"
    signal = "{{.SIGNAL_ID}}"
  }
}
