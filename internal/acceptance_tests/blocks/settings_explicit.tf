resource "fastly_service_settings" "test" {
  service_id         = fastly_service_cdn.test.id
  version            = {{.SERVICE_VERSION}}
  default_host       = "override.example.com"
  default_ttl        = 120
  http3              = true
  stale_if_error     = true
  stale_if_error_ttl = 600
}
