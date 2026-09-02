resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_virtual_patch" "test" {
  workspace_id      = fastly_ngwaf_workspace.test.id
  virtual_patch_id  = "{{.VIRTUAL_PATCH_ID}}"
  mode              = "log"
  enabled           = false
}
