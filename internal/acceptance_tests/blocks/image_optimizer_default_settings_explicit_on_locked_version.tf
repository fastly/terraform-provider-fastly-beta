resource "fastly_service_image_optimizer_default_settings" "locked" {
  service_id = fastly_service_cdn.test.id
  version    = 2
  webp       = true
}
