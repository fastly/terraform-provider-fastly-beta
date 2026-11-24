resource "fastly_service_vcl" "traffic" {
  name = "traffic-migration"

  domain {
    name = "traffic.example.com"
  }

  # Secondary origin.
  backend {
    name              = "origin_b"
    address           = "b.example.com"
    port              = 443
    use_ssl           = true
    ssl_cert_hostname = "b.example.com"
    healthcheck       = "health_b"
  }

  backend {
    name              = "origin_a"
    address           = "a.example.com"
    port              = 443
    use_ssl           = true
    ssl_cert_hostname = "a.example.com"
    healthcheck       = "health_a"
  }

  backend {
    name              = "origin_c"
    address           = "c.example.com"
    port              = 443
    use_ssl           = true
    ssl_cert_hostname = "c.example.com"
  }

  healthcheck {
    name    = "health_b"
    host    = "b.example.com"
    path    = "/healthz"
    headers = ["X-Probe: 1"]
  }

  healthcheck {
    name           = "health_a"
    host           = "a.example.com"
    path           = "/status"
    method         = "GET"
    check_interval = 15000
  }

  condition {
    name      = "is_api"
    statement = "req.url ~ \"^/api/\""
    type      = "REQUEST"
    priority  = 5
  }

  condition {
    name      = "cache_static"
    statement = "req.url.ext ~ \"(?i)^(css|js)$\""
    type      = "CACHE"
  }

  condition {
    name      = "is_error"
    statement = "beresp.status >= 500"
    type      = "CACHE"
  }

  condition {
    name      = "is_missing"
    statement = "resp.status == 404"
    type      = "RESPONSE"
  }

  director {
    name     = "pool_random"
    backends = ["origin_a", "origin_b"]
  }

  director {
    name     = "pool_hash"
    backends = ["origin_a", "origin_b", "origin_c"]
    type     = 3 # consistent hashing
    quorum   = 50
  }

  director {
    name     = "pool_client"
    backends = ["origin_c"]
    type     = 4
    retries  = 2
  }

  director {
    name     = "pool_explicit_random"
    backends = ["origin_a"]
    type     = 1
  }

  header {
    name              = "strip server"
    type              = "response"
    action            = "delete"
    destination       = "http.Server"
    response_condition = ""
  }

  header {
    name              = "api origin"
    type              = "request"
    action            = "set"
    destination       = "http.X-Origin"
    source            = "\"api\""
    request_condition = "is_api"
    priority          = 10
  }

  gzip {
    name = "defaults"
  }

  gzip {
    name            = "assets"
    extensions      = ["css", "js"]
    content_types   = ["text/css", "application/javascript"]
    cache_condition = "cache_static"
  }

  gzip {
    name            = "blank_condition"
    extensions      = ["html"]
    cache_condition = ""
  }

  cache_setting {
    name            = "static"
    action          = "CACHE"
    cache_condition = "cache_static"
    ttl             = 3600
  }

  cache_setting {
    name            = "errors"
    action          = "pass"
    cache_condition = "is_error"
  }

  cache_setting {
    name            = "ttl_only"
    action          = ""
    cache_condition = ""
    stale_ttl       = 60
  }

  request_setting {
    name              = "api"
    action            = "PASS"
    xff               = "Append"
    request_condition = "is_api"
  }

  request_setting {
    name      = "force_tls"
    action    = ""
    xff       = ""
    force_ssl = true
  }

  response_object {
    name              = "not_found"
    status            = 404
    response          = "Not Found"
    content           = "missing"
    content_type      = "text/plain"
    request_condition = "is_api"
  }

  response_object {
    name            = "error_page"
    status          = 503
    response        = "Unavailable"
    content_type    = ""
    cache_condition = "is_error"
  }

  force_destroy = true
}
