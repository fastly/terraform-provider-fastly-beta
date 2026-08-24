  response_object {
    name = "{{.RESPONSE_OBJECT_NAME}}"
  }

  rate_limiter {
    name                 = "{{.RATE_LIMITER_NAME}}"
    action               = "response_object"
    client_key           = ["req.http.Fastly-Client-IP"]
    http_methods         = ["GET"]
    penalty_box_duration = 5
    response_object_name = "{{.RESPONSE_OBJECT_NAME}}"
    rps_limit            = 50
    window_size          = 60
  }
