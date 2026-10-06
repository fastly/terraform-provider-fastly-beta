resource "fastly_service_header" "test" {
  service_id  = fastly_service_cdn.test.id
  version     = {{.SERVICE_VERSION}}
  name        = "{{.HEADER_NAME}}"
  action      = "delete"
  type        = "cache"
  destination = "http.aws-id"
}
