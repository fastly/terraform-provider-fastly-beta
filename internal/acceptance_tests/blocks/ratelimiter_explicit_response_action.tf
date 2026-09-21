resource "fastly_service_ratelimiter" "test" {
  service_id           = fastly_service_cdn.test.id
  version              = {{.SERVICE_VERSION}}
  name                 = "{{.RATE_LIMITER_NAME}}"
  action               = "response"
  client_key           = ["req.http.Fastly-Client-IP"]
  http_methods         = ["GET"]
  penalty_box_duration = 10
  rps_limit            = 100
  window_size          = 60

  response = {
    content      = "Rate limit exceeded"
    content_type = "text/plain"
    status       = 429
  }
}
