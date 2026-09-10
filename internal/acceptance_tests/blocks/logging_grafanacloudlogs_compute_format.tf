resource "fastly_service_logging_grafanacloudlogs" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_GRAFANACLOUDLOGS_NAME}}"
  url        = "https://test123.grafana.net"
  user       = "123456"
  index      = "{\"label\": \"value\"}"
  authentication = {
    token = "test-grafanacloudlogs-token"
  }
  format = "%h %l %u %t \"%r\" %>s %b"
}
