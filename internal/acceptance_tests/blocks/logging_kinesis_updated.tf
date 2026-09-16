resource "fastly_service_logging_kinesis" "test" {
  service_id        = fastly_service_cdn.test.id
  version           = {{.SERVICE_VERSION}}
  name              = "{{.LOGGING_KINESIS_NAME}}"
  topic             = "updated-stream"
  region            = "us-west-2"
  processing_region = "eu"

  authentication = {
    access_key = "AKIAI44QH8DHBEXAMPLE"
    secret_key = "je7MtGbClwBF/2Zp9Utk/h3yCo8nvbEXAMPLEKEY"
  }

  format         = "%h %l %u %t \"%r\" %>s %b"
  format_version = 2
  placement      = "none"
}
