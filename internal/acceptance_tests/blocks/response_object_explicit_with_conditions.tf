resource "fastly_service_condition" "request" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.REQUEST_CONDITION_NAME}}"
  type       = "REQUEST"
  statement  = "req.url ~ \"^/admin\""
}

resource "fastly_service_condition" "cache" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.CACHE_CONDITION_NAME}}"
  type       = "CACHE"
  statement  = "beresp.status == 200"
}

resource "fastly_service_response_object" "test" {
  service_id        = fastly_service_cdn.test.id
  version           = {{.SERVICE_VERSION}}
  name              = "{{.RESPONSE_OBJECT_NAME}}"
  request_condition = fastly_service_condition.request.name
  cache_condition   = fastly_service_condition.cache.name
  status            = 503
  response          = "Service Unavailable"
  content           = "Temporarily unavailable"
  content_type      = "text/plain"
}
