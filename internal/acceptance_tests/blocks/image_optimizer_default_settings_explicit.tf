resource "fastly_service_image_optimizer_default_settings" "test" {
  service_id    = fastly_service_cdn.test.id
  version       = {{.SERVICE_VERSION}}
  resize_filter = "bicubic"
  webp          = true
  webp_quality  = 70
  jpeg_type     = "progressive"
  jpeg_quality  = 90
  upscale       = true
  allow_video   = true

  depends_on = [fastly_service_product_image_optimizer.image_optimizer]
}
