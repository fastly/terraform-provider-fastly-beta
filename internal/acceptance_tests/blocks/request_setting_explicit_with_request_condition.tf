resource "fastly_service_condition" "request" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.CONDITION_NAME}}"
  type       = "REQUEST"
  statement  = "req.url ~ \"^/api/\""
}

resource "fastly_service_request_setting" "test" {
  service_id        = fastly_service_cdn.test.id
  version           = {{.SERVICE_VERSION}}
  name              = "{{.REQUEST_SETTING_NAME}}"
  action            = "pass"
  request_condition = fastly_service_condition.request.name
}
