resource "fastly_service_logging_ftp" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_FTP_NAME}}"
  address    = "ftp.example.com"
  path       = "/"
  authentication = {
    user     = "test-user"
    password = "test-password"
  }
  timestamp_format = ""
}
