resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_rate_limit_rule" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  description  = "Rate limit rule without its client_identifiers block"
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
  }

  action {
    type   = "log_request"
    signal = "site.tf-test-signal"
  }
}
