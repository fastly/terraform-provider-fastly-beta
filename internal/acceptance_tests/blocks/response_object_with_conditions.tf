  condition {
    name      = "{{.REQUEST_CONDITION_NAME}}"
    type      = "REQUEST"
    statement = "req.url ~ \"^/admin\""
  }

  condition {
    name      = "{{.CACHE_CONDITION_NAME}}"
    type      = "CACHE"
    statement = "req.url ~ \"\\.(css|js|html)$\""
  }

  response_object {
    name              = "{{.RESPONSE_OBJECT_NAME}}"
    request_condition = "{{.REQUEST_CONDITION_NAME}}"
    cache_condition   = "{{.CACHE_CONDITION_NAME}}"
  }
