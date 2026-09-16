logging_kinesis {
  name      = "{{.LOGGING_KINESIS_NAME}}"
  topic     = "test-stream"
  placement = "none"

  authentication = {
    access_key = "AKIAIOSFODNN7EXAMPLE"
    secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  }
}
