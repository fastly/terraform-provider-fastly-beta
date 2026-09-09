logging_grafanacloudlogs {
  name = "{{.LOGGING_GRAFANACLOUDLOGS_NAME}}"
  url  = "https://test123.grafana.net"
  user = "123456"
  index = "{\"label\": \"value\"}"
  authentication = {
    token = "test-grafanacloudlogs-token"
  }
  placement = "none"
}
