logging_sftp {
  name            = "{{.LOGGING_SFTP_NAME_1}}"
  address         = "sftp.example.com"
  path            = "/"
  ssh_known_hosts = "sftp.example.com"
  authentication = {
    user     = "test-user"
    password = "test-password"
  }
}

logging_sftp {
  name            = "{{.LOGGING_SFTP_NAME_2}}"
  address         = "sftp.example.com"
  path            = "/"
  ssh_known_hosts = "sftp.example.com"
  authentication = {
    user     = "test-user"
    password = "test-password"
  }
}
