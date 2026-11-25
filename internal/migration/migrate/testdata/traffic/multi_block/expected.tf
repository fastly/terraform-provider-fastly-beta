variable "cache_action" {
  type    = string
  default = "PASS"
}

variable "request_action" {
  type    = string
  default = null
}

variable "request_xff" {
  type    = string
  default = "Append"
}

variable "gzip_extensions" {
  type    = list(string)
  default = []
}

variable "gzip_content_types" {
  type    = list(string)
  default = ["text/plain"]
}

resource "fastly_service_cdn_auto" "traffic" {
  name = "traffic-migration"

  domain {
    name = "traffic.example.com"
  }

  backend {
    name              = "origin_a"
    address           = "a.example.com"
    port              = 443
    use_ssl           = true
    ssl_cert_hostname = "a.example.com"
    healthcheck       = "health_a"
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
    name              = "origin_c"
    address           = "c.example.com"
    port              = 443
    use_ssl           = true
    ssl_cert_hostname = "c.example.com"
  }

  healthcheck {
    name           = "health_a"
    host           = "a.example.com"
    path           = "/status"
    method         = "GET"
    check_interval = 15000
  }

  healthcheck {
    name    = "health_b"
    host    = "b.example.com"
    path    = "/healthz"
    headers = ["X-Probe: 1"]
  }

  condition {
    name      = "cache_static"
    statement = "req.url.ext ~ \"(?i)^(css|js)$\""
    type      = "CACHE"
  }

  condition {
    name      = "is_api"
    statement = "req.url ~ \"^/api/\""
    type      = "REQUEST"
    priority  = 5
  }

  condition {
    name      = "is_error"
    statement = "beresp.status >= 500"
    type      = "CACHE"
  }

  condition {
    name      = "is_internal"
    statement = "req.url ~ \"^/internal/\""
    type      = "REQUEST"
  }

  condition {
    name      = "is_missing"
    statement = "resp.status == 404"
    type      = "RESPONSE"
  }

  condition {
    name      = "is_redirect"
    statement = "beresp.status == 301"
    type      = "CACHE"
  }

  director {
    name     = "pool_client"
    backends = ["origin_c"]
    type     = "client"
    retries  = 2
  }

  director {
    name     = "pool_explicit_random"
    backends = ["origin_a"]
    type     = "random"
  }

  director {
    name     = "pool_hash"
    backends = ["origin_a", "origin_b", "origin_c"]
    type     = "hash" # consistent hashing
    quorum   = 50
  }

  director {
    name     = "pool_random"
    backends = ["origin_a", "origin_b"]
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

  header {
    name              = "strip server"
    type              = "response"
    action            = "delete"
    destination       = "http.Server"
    response_condition = ""
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
  }

  gzip {
    name = "defaults"
  }

  gzip {
    name          = "empty_lists"
  }

  gzip {
    name          = "variable_lists"
    content_types = var.gzip_content_types == null ? null : (length(var.gzip_content_types) == 0 ? null : var.gzip_content_types)
    extensions    = var.gzip_extensions == null ? null : (length(var.gzip_extensions) == 0 ? null : var.gzip_extensions)
  }

  cache_setting {
    name            = "errors"
    action          = "pass"
    cache_condition = "is_error"
  }

  cache_setting {
    name            = "static"
    action          = "cache"
    cache_condition = "cache_static"
    ttl             = 3600
  }

  cache_setting {
    name            = "ttl_only"
    stale_ttl       = 60
  }

  cache_setting {
    name            = "variable_action"
    action          = var.cache_action == null ? null : lower(var.cache_action)
    cache_condition = "is_redirect"
  }

  request_setting {
    name              = "api"
    action            = "pass"
    xff               = "append"
    request_condition = "is_api"
  }

  request_setting {
    name      = "force_tls"
    force_ssl = true
  }

  request_setting {
    name              = "variable_xff"
    action            = var.request_action == null ? null : lower(var.request_action)
    xff               = var.request_xff == null ? null : lower(var.request_xff)
    request_condition = "is_internal"
  }

  response_object {
    name            = "error_page"
    status          = 503
    response        = "Unavailable"
    cache_condition = "is_error"
  }

  response_object {
    name              = "not_found"
    status            = 404
    response          = "Not Found"
    content           = "missing"
    content_type      = "text/plain"
    request_condition = "is_api"
  }

  force_destroy = true
}
