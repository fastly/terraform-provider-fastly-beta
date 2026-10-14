resource "fastly_service_response_object" "locked" {
  service_id = fastly_service_cdn.test.id
  version    = 2
  name       = "{{.RESPONSE_OBJECT_NAME}}"
  status     = 503
  response   = "Service Unavailable"
}
