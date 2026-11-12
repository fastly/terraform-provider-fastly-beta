resource "fastly_alert" "test" {
  name   = "{{.ALERT_NAME}}"
  source = "stats"
  metric = "status_5xx"

  dimensions {
    services = {{.SERVICES}}
  }

  evaluation_strategy {
    type      = "above_threshold"
    period    = "5m"
    threshold = 10
  }
}
