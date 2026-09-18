resource "fastly_service_logging_papertrail" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_PAPERTRAIL_NAME}}"
  address    = "logs.papertrailapp.com"
  port       = 12345
  format     = "%h %l %u %t \"%r\" %>s %b"
}
