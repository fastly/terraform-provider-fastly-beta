resource "fastly_service_logging_kafka" "test" {
  service_id        = fastly_service_cdn.test.id
  version           = {{.SERVICE_VERSION}}
  name              = "{{.LOGGING_KAFKA_NAME}}"
  brokers           = "127.0.0.1:9092,127.0.0.2:9092"
  topic             = "updated-topic"
  compression_codec = "snappy"
  required_acks     = "-1"
  request_max_bytes = 12345
  parse_log_keyvals = true
  auth_method       = "scram-sha-256"
  processing_region = "eu"
  use_tls           = true

  authentication = {
    user     = "kafka-user"
    password = "kafka-password"
  }

  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
}
