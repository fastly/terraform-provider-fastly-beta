resource "fastly_service_request_setting" "test" {
  service_id       = fastly_service_cdn.test.id
  version          = {{.SERVICE_VERSION}}
  name             = "{{.REQUEST_SETTING_NAME}}"
  action           = "lookup"
  bypass_busy_wait = true
  default_host     = "host.example.com"
  force_miss       = true
  force_ssl        = true
  hash_keys        = "req.url,req.http.host"
  max_stale_age    = 120
  timer_support    = true
  xff              = "append"
}
