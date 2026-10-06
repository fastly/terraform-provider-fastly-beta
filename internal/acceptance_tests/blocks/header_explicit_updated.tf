resource "fastly_service_header" "test" {
  service_id    = fastly_service_cdn.test.id
  version       = {{.SERVICE_VERSION}}
  name          = "{{.HEADER_NAME}}"
  action        = "set"
  type          = "request"
  destination   = "http.X-Custom"
  ignore_if_set = true
  priority      = 10
  source        = "http.server-name"
}
