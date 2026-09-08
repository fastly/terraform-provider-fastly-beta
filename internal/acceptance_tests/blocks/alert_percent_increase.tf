resource "fastly_alert" "test" {
  name   = "{{.ALERT_NAME}}"
  source = "stats"
  metric = "status_4xx"

  evaluation_strategy {
    type         = "percent_increase"
    period       = "2m"
    threshold    = {{.THRESHOLD}}
    ignore_below = {{.IGNORE_BELOW}}
  }
}
