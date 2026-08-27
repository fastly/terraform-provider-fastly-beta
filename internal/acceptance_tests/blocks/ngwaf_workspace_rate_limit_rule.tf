resource "fastly_ngwaf_workspace_rate_limit_rule" "test" {
  workspace_id = "{{.WORKSPACE_ID}}"
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
  }

  action {
    type   = "log_request"
    signal = "{{.SIGNAL_ID}}"
  }
}
