resource "fastly_service_ratelimiter" "test" {
  service_id           = fastly_service_cdn.test.id
  version              = {{.SERVICE_VERSION}}
  name                 = "{{.RATE_LIMITER_NAME}}"
  action               = "log_only"
  client_key           = ["req.http.Fastly-Client-IP", "req.http.User-Agent"]
  http_methods         = ["GET", "POST", "PUT"]
  logger_type          = "s3"
  penalty_box_duration = 20
  rps_limit            = 500
  window_size          = 10
}
