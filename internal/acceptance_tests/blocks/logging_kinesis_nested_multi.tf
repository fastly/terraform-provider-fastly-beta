logging_kinesis {
  name  = "{{.LOGGING_KINESIS_NAME_1}}"
  topic = "test-stream"

  authentication = {
    access_key = "AKIAIOSFODNN7EXAMPLE"
    secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  }
}

logging_kinesis {
  name  = "{{.LOGGING_KINESIS_NAME_2}}"
  topic = "test-stream"

  authentication = {
    access_key = "AKIAIOSFODNN7EXAMPLE"
    secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  }
}
