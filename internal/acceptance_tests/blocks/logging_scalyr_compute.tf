resource "fastly_service_logging_scalyr" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_SCALYR_NAME}}"
  authentication = {
    token = "test-scalyr-token"
  }
}
