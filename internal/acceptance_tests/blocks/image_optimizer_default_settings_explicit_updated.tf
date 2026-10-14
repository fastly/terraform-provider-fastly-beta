resource "fastly_service_image_optimizer_default_settings" "test" {
  service_id    = fastly_service_cdn.test.id
  version       = {{.SERVICE_VERSION}}
  resize_filter = "nearest"
  webp          = false
  webp_quality  = 50
  jpeg_type     = "baseline"
  jpeg_quality  = 60
  upscale       = false
  allow_video   = false

  depends_on = [fastly_service_product_image_optimizer.image_optimizer]
}
