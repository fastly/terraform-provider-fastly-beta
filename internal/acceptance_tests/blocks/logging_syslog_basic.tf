resource "fastly_service_logging_syslog" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_SYSLOG_NAME}}"
  address    = "syslog.example.com"
}
