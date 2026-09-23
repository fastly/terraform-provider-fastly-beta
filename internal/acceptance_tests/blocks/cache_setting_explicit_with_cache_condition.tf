resource "fastly_service_condition" "cache" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.CONDITION_NAME}}"
  type       = "CACHE"
  statement  = "beresp.status == 200"
}

resource "fastly_service_cache_setting" "test" {
  service_id      = fastly_service_cdn.test.id
  version         = {{.SERVICE_VERSION}}
  name            = "{{.CACHE_SETTING_NAME}}"
  action          = "cache"
  ttl             = 3600
  cache_condition = fastly_service_condition.cache.name
}
