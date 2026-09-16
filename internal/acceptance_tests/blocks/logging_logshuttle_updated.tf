resource "fastly_service_logging_logshuttle" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_LOGSHUTTLE_NAME}}"
  url        = "https://west.logplex.io/logs"
  authentication = {
    token = "updated-logshuttle-token"
  }
  processing_region = "eu"
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
