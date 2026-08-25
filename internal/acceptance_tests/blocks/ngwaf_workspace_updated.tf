resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace Updated"
  mode        = "log"

  ip_anonymization  = "hashed"
  client_ip_headers = ["True-Client-IP"]

  default_blocking_response_code = 301
  default_redirect_url           = "https://example.com"

  attack_signal_thresholds {
    one_minute  = 200
    ten_minutes = 1000
    one_hour    = 2000
    immediate   = false
  }
}
