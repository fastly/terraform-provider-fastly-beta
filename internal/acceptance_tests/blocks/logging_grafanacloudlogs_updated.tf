resource "fastly_service_logging_grafanacloudlogs" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_GRAFANACLOUDLOGS_NAME}}"
  url        = "https://test456.grafana.net"
  user       = "987654"
  index      = "{\"label2\": \"value2\"}"
  authentication = {
    token = "updated-grafanacloudlogs-token"
  }
  processing_region = "eu"
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
