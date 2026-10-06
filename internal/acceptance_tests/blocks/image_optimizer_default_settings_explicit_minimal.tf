resource "fastly_service_image_optimizer_default_settings" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}

  depends_on = [fastly_service_product_image_optimizer.image_optimizer]
}
