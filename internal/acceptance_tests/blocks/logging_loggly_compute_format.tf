resource "fastly_service_logging_loggly" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_LOGGLY_NAME}}"
  authentication = {
    token = "test-loggly-token"
  }
  format = "%h %l %u %t \"%r\" %>s %b"
}
