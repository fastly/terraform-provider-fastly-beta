resource "fastly_service_settings" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
}
