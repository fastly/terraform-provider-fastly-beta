resource "fastly_service_condition" "cache" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.CONDITION_NAME}}"
  type       = "CACHE"
  statement  = "beresp.status == 200"
}

resource "fastly_service_gzip" "test" {
  service_id      = fastly_service_cdn.test.id
  version         = {{.SERVICE_VERSION}}
  name            = "{{.GZIP_NAME}}"
  cache_condition = fastly_service_condition.cache.name
}
