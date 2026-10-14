resource "fastly_service_request_setting" "test" {
  service_id       = fastly_service_cdn.test.id
  version          = {{.SERVICE_VERSION}}
  name             = "{{.REQUEST_SETTING_NAME}}"
  action           = "pass"
  bypass_busy_wait = false
  default_host     = "other.example.com"
  force_miss       = false
  force_ssl        = false
  hash_keys        = "req.http.host"
  max_stale_age    = 300
  timer_support    = false
  xff              = "clear"
}
