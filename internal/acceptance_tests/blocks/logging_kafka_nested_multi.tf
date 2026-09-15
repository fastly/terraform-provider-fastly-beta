logging_kafka {
  name    = "{{.LOGGING_KAFKA_NAME_1}}"
  brokers = "127.0.0.1:9092"
  topic   = "test-topic"
}

logging_kafka {
  name    = "{{.LOGGING_KAFKA_NAME_2}}"
  brokers = "127.0.0.1:9092"
  topic   = "test-topic"
}
