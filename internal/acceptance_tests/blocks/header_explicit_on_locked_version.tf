resource "fastly_service_header" "locked" {
  service_id  = fastly_service_cdn.test.id
  version     = 2
  name        = "{{.HEADER_NAME}}"
  action      = "delete"
  type        = "cache"
  destination = "http.aws-id"
}
