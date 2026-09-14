resource "fastly_service_logging_elasticsearch" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_ELASTICSEARCH_NAME}}"
  index      = "logs-index"
  url        = "https://elasticsearch.example.com"
  format     = "%h %l %u %t \"%r\" %>s %b"
}
