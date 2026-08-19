resource "fastly_service_logging_syslog" "test" {
  service_id        = fastly_service_cdn.test.id
  version           = {{.SERVICE_VERSION}}
  name              = "{{.LOGGING_SYSLOG_NAME}}"
  address           = "syslog-updated.example.com"
  port              = 9000
  message_type      = "loggly"
  processing_region = "eu"
  use_tls           = true
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
