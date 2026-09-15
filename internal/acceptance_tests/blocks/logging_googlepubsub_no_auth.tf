resource "fastly_service_logging_googlepubsub" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_GOOGLEPUBSUB_NAME}}"
  project_id = "fastly-test-project"
  topic      = "fastly-test-topic"
}
