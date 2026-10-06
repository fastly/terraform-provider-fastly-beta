resource "fastly_service_request_setting" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.REQUEST_SETTING_NAME}}"
}
