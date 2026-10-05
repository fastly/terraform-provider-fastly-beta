resource "fastly_service_healthcheck" "locked" {
  service_id = fastly_service_cdn.test.id
  version    = 2
  name       = "{{.HEALTHCHECK_NAME}}"
  host       = "example.com"
  path       = "/healthz"
}
