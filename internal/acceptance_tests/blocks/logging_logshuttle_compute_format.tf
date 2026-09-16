resource "fastly_service_logging_logshuttle" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_LOGSHUTTLE_NAME}}"
  url        = "https://east.logplex.io/logs"
  authentication = {
    token = "test-logshuttle-token"
  }
  format = "%h %l %u %t \"%r\" %>s %b"
}
