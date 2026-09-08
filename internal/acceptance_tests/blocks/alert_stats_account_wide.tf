resource "fastly_alert" "test" {
  name        = "{{.ALERT_NAME}}"
  description = "{{.ALERT_DESCRIPTION}}"
  source      = "stats"
  metric      = "{{.METRIC}}"

  evaluation_strategy {
    type      = "{{.EVAL_TYPE}}"
    period    = "{{.EVAL_PERIOD}}"
    threshold = {{.EVAL_THRESHOLD}}
  }
}
