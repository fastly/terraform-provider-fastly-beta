resource "fastly_alert" "test" {
  name   = "{{.ALERT_NAME}}"
  source = "domains"
  metric = "{{.METRIC}}"

  evaluation_strategy {
    type      = "above_threshold"
    period    = "5m"
    threshold = 10
  }
}
