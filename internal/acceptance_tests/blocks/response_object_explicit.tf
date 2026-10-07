resource "fastly_service_response_object" "test" {
  service_id   = fastly_service_cdn.test.id
  version      = {{.SERVICE_VERSION}}
  name         = "{{.RESPONSE_OBJECT_NAME}}"
  status       = 200
  response     = "OK"
  content      = "test content"
  content_type = "text/html"
}
