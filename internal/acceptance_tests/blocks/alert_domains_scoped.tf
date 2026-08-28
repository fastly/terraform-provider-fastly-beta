resource "fastly_alert" "test" {
  name        = "{{.ALERT_NAME}}"
  description = "{{.ALERT_DESCRIPTION}}"
  service_id  = {{.SERVICE_ID_REF}}
  source      = "domains"
  metric      = "{{.METRIC}}"

  dimensions {
    domains = ["{{.DOMAIN_NAME}}"]
  }

  evaluation_strategy {
    type      = "{{.EVAL_TYPE}}"
    period    = "{{.EVAL_PERIOD}}"
    threshold = {{.EVAL_THRESHOLD}}
  }
}
