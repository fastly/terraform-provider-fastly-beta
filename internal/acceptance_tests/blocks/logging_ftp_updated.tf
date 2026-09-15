resource "fastly_service_logging_ftp" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_FTP_NAME}}"
  address    = "ftp.example.com"
  path       = "/logs/"
  authentication = {
    user     = "test-user"
    password = "updated-password"
  }
  port              = 2121
  processing_region = "eu"
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
