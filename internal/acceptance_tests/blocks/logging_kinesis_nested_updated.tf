logging_kinesis {
  name              = "{{.LOGGING_KINESIS_NAME}}"
  topic             = "updated-stream"
  region            = "us-west-2"
  processing_region = "eu"
  format_version    = 2

  authentication = {
    access_key = "AKIAI44QH8DHBEXAMPLE"
    secret_key = "je7MtGbClwBF/2Zp9Utk/h3yCo8nvbEXAMPLEKEY"
  }
}
