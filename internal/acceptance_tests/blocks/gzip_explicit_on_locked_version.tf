resource "fastly_service_gzip" "locked" {
  service_id = fastly_service_cdn.test.id
  version    = 2
  name       = "{{.GZIP_NAME}}"
}
