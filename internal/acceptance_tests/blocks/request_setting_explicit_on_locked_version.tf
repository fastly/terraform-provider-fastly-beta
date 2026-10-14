resource "fastly_service_request_setting" "locked" {
  service_id = fastly_service_cdn.test.id
  version    = 2
  name       = "{{.REQUEST_SETTING_NAME}}"
  action     = "lookup"
}
