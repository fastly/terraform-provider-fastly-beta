logging_kafka {
  name              = "{{.LOGGING_KAFKA_NAME}}"
  brokers           = "127.0.0.1:9092,127.0.0.2:9092"
  topic             = "updated-topic"
  compression_codec = "snappy"
  required_acks     = "-1"
  processing_region = "eu"
  format_version    = 2
}
