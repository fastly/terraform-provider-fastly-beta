resource "fastly_service_condition" "cache" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.CONDITION_NAME}}"
  type       = "CACHE"
  statement  = "beresp.status == 200"
}

resource "fastly_service_header" "test" {
  service_id      = fastly_service_cdn.test.id
  version         = {{.SERVICE_VERSION}}
  name            = "{{.HEADER_NAME}}"
  action          = "delete"
  type            = "cache"
  destination     = "http.aws-id"
  cache_condition = fastly_service_condition.cache.name
}
