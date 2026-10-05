resource "fastly_service_healthcheck" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.HEALTHCHECK_NAME}}"
  host       = "example.com"
  path       = "/healthz"
}
