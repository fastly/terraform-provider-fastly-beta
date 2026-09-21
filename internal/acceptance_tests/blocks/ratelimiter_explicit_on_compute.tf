resource "fastly_service_ratelimiter" "test" {
  service_id           = fastly_service_compute.test.id
  version              = {{.SERVICE_VERSION}}
  name                 = "{{.RATE_LIMITER_NAME}}"
  action               = "log_only"
  client_key           = ["req.http.Fastly-Client-IP"]
  http_methods         = ["GET"]
  logger_type          = "s3"
  penalty_box_duration = 10
  rps_limit            = 100
  window_size          = 60
}
