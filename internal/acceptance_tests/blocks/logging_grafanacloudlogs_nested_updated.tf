logging_grafanacloudlogs {
  name = "{{.LOGGING_GRAFANACLOUDLOGS_NAME}}"
  url  = "https://test456.grafana.net"
  user = "987654"
  index = "{\"label2\": \"value2\"}"
  authentication = {
    token = "updated-grafanacloudlogs-token"
  }
  processing_region = "eu"
  format_version    = 2
}
