resource "fastly_service_settings" "test" {
  service_id         = fastly_service_cdn.test.id
  version            = {{.SERVICE_VERSION}}
  default_host       = "other.example.com"
  default_ttl        = 300
  http3              = false
  stale_if_error     = false
  stale_if_error_ttl = 1200
}
