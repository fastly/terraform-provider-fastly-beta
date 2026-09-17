logging_scalyr {
  name = "{{.LOGGING_SCALYR_NAME}}"
  authentication = {
    token = "test-scalyr-token"
  }
  format = "%h %l %u %t \"%r\" %>s %b"
}
