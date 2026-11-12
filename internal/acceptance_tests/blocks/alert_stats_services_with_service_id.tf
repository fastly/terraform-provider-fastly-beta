resource "fastly_alert" "test" {
  name       = "{{.ALERT_NAME}}"
  service_id = "abc123"
  source     = "stats"
  metric     = "status_5xx"

  dimensions {
    services = ["abc123"]
  }

  evaluation_strategy {
    type      = "above_threshold"
    period    = "5m"
    threshold = 10
  }
}
