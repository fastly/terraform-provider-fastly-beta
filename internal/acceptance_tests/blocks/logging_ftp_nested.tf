logging_ftp {
  name    = "{{.LOGGING_FTP_NAME}}"
  address = "ftp.example.com"
  path    = "/"
  authentication = {
    user     = "test-user"
    password = "test-password"
  }
}
