resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace"
  mode        = "block"

  ip_anonymization  = "hashed"
  client_ip_headers = ["X-Forwarded-For", "X-Real-IP"]

  default_blocking_response_code = 429

  auto_enable_new_virtual_patches = true

  attack_signal_thresholds {
    one_minute  = 100
    ten_minutes = 500
    one_hour    = 1000
    immediate   = true
  }
}
