resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_threshold" "one" {
  workspace_id = fastly_ngwaf_workspace.test.id

  action      = "block"
  dont_notify = false
  enabled     = true
  interval    = 3600
  limit       = 10
  name        = "{{.THRESHOLD_NAME}}_1"
  signal      = "SQLI"
}

resource "fastly_ngwaf_workspace_threshold" "two" {
  workspace_id = fastly_ngwaf_workspace.test.id

  action      = "log"
  dont_notify = true
  enabled     = false
  interval    = 600
  limit       = 50
  name        = "{{.THRESHOLD_NAME}}_2"
  signal      = "BHH"
}

data "fastly_ngwaf_workspace_thresholds" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id

  depends_on = [fastly_ngwaf_workspace_threshold.one, fastly_ngwaf_workspace_threshold.two]
}
