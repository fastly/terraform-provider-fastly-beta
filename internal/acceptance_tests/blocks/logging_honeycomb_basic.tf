resource "fastly_service_logging_honeycomb" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_HONEYCOMB_NAME}}"
  dataset    = "test-dataset"
  authentication = {
    token = "test-honeycomb-key"
  }
}
