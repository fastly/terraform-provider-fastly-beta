resource "fastly_service_response_object" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.RESPONSE_OBJECT_NAME}}"
}
