resource "fastly_service_cache_setting" "locked" {
  service_id = fastly_service_cdn.test.id
  version    = 2
  name       = "{{.CACHE_SETTING_NAME}}"
  action     = "cache"
  ttl        = 3600
}
