logging_grafanacloudlogs {
  name = "{{.LOGGING_GRAFANACLOUDLOGS_NAME_1}}"
  url  = "https://test123.grafana.net"
  user = "123456"
  index = "{\"label\": \"value\"}"
  authentication = {
    token = "test-grafanacloudlogs-token"
  }
}

logging_grafanacloudlogs {
  name = "{{.LOGGING_GRAFANACLOUDLOGS_NAME_2}}"
  url  = "https://test123.grafana.net"
  user = "123456"
  index = "{\"label\": \"value\"}"
  authentication = {
    token = "test-grafanacloudlogs-token"
  }
}
