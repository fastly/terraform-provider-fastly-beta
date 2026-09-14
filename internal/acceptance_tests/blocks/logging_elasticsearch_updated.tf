resource "fastly_service_logging_elasticsearch" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_ELASTICSEARCH_NAME}}"
  index      = "logs-index-updated"
  url        = "https://elasticsearch-updated.example.com"
  pipeline   = "my-pipeline"

  authentication = {
    user     = "es-user"
    password = "es-password"
  }

  processing_region   = "eu"
  request_max_bytes   = 1000
  request_max_entries = 100
  format               = "%h %l %u %t \"%r\" %>s %b"
  format_version       = 2
  placement            = "none"
}
