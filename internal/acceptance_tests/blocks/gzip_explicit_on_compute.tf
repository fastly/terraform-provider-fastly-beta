resource "fastly_service_gzip" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.GZIP_NAME}}"
}
