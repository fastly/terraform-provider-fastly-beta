resource "fastly_service_cache_setting" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.CACHE_SETTING_NAME}}"
  action     = "pass"
  ttl        = 7200
  stale_ttl  = 300
}
