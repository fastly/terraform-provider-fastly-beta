logging_ftp {
  name    = "{{.LOGGING_FTP_NAME_1}}"
  address = "ftp.example.com"
  path    = "/"
  authentication = {
    user     = "test-user"
    password = "test-password"
  }
}

logging_ftp {
  name    = "{{.LOGGING_FTP_NAME_2}}"
  address = "ftp.example.com"
  path    = "/"
  authentication = {
    user     = "test-user"
    password = "test-password"
  }
}
