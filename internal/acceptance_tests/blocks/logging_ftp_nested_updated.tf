logging_ftp {
  name    = "{{.LOGGING_FTP_NAME}}"
  address = "ftp.example.com"
  path    = "/"
  authentication = {
    user     = "test-user"
    password = "updated-password"
  }
  port              = 2121
  processing_region = "eu"
  format_version    = 2
}
