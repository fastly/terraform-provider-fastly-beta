resource "fastly_service_logging_loggly" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_LOGGLY_NAME}}"
  authentication = {
    token = "updated-loggly-token"
  }
  processing_region = "eu"
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
