resource "fastly_service_logging_sftp" "test" {
  service_id      = fastly_service_cdn.test.id
  version         = {{.SERVICE_VERSION}}
  name            = "{{.LOGGING_SFTP_NAME}}"
  address         = "sftp.example.com"
  path            = "/logs/"
  ssh_known_hosts = "sftp.example.com"
  authentication = {
    user     = "test-user"
    password = "updated-password"
  }
  port              = 2222
  processing_region = "eu"
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
