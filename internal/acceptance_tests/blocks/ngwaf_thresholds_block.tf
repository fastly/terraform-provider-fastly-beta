resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_thresholds" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  action       = "block"
  dont_notify  = false
  duration     = 86400
  enabled      = true
  interval     = 3600
  limit        = 10
  name         = "{{.THRESHOLD_NAME}}"
  signal       = "SQLI"
}
