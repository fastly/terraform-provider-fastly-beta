resource "fastly_service_logging_kafka" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_KAFKA_NAME}}"
  brokers    = "127.0.0.1:9092"
  topic      = "test-topic"
  format     = "%h %l %u %t \"%r\" %>s %b"
}
