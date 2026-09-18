logging_sftp {
  name            = "{{.LOGGING_SFTP_NAME}}"
  address         = "sftp.example.com"
  path            = "/"
  ssh_known_hosts = "sftp.example.com"
  authentication = {
    user     = "test-user"
    password = "updated-password"
  }
  port              = 2222
  processing_region = "eu"
  format_version    = 2
}
