resource "fastly_service_image_optimizer_default_settings" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
}
