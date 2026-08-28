resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_thresholds" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  action       = "log"
  dont_notify  = true
  duration     = 43200
  enabled      = false
  interval     = 600
  limit        = 50
  name         = "{{.THRESHOLD_NAME}}"
  signal       = "BHH"
}
