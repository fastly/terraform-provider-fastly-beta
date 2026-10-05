resource "fastly_service_header" "test" {
  service_id    = fastly_service_cdn.test.id
  version       = {{.SERVICE_VERSION}}
  name          = "{{.HEADER_NAME}}"
  action        = "regex"
  type          = "request"
  destination   = "http.X-Custom"
  ignore_if_set = true
  priority      = 10
  regex         = "^foo"
  source        = "http.server-name"
  substitution  = "bar"
}
