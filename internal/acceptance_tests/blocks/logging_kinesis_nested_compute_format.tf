logging_kinesis {
  name   = "{{.LOGGING_KINESIS_NAME}}"
  topic  = "test-stream"
  format = "%h %l %u %t \"%r\" %>s %b"

  authentication = {
    access_key = "AKIAIOSFODNN7EXAMPLE"
    secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  }
}
