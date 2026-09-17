resource "fastly_service_logging_sftp" "test" {
  service_id      = fastly_service_compute.test.id
  version         = {{.SERVICE_VERSION}}
  name            = "{{.LOGGING_SFTP_NAME}}"
  address         = "sftp.example.com"
  path            = "/"
  ssh_known_hosts = "sftp.example.com"
  authentication = {
    user     = "test-user"
    password = "test-password"
  }
  format = "%h %l %u %t \"%r\" %>s %b"
}
