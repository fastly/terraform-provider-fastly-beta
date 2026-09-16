logging_loggly {
  name = "{{.LOGGING_LOGGLY_NAME}}"
  authentication = {
    token = "test-loggly-token"
  }
  format = "%h %l %u %t \"%r\" %>s %b"
}
