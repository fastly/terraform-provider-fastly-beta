resource "fastly_service_cache_setting" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.CACHE_SETTING_NAME}}"
  action     = "cache"
  ttl        = 3600
}
