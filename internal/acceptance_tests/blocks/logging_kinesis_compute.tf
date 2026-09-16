resource "fastly_service_logging_kinesis" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_KINESIS_NAME}}"
  topic      = "test-stream"

  authentication = {
    access_key = "AKIAIOSFODNN7EXAMPLE"
    secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  }
}
