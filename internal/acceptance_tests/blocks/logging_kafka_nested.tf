logging_kafka {
  name    = "{{.LOGGING_KAFKA_NAME}}"
  brokers = "127.0.0.1:9092"
  topic   = "test-topic"
}
