resource "fastly_service_logging_papertrail" "test" {
  service_id        = fastly_service_cdn.test.id
  version           = {{.SERVICE_VERSION}}
  name              = "{{.LOGGING_PAPERTRAIL_NAME}}"
  address           = "logs2.papertrailapp.com"
  port              = 54321
  processing_region = "eu"
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
