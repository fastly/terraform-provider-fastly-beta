resource "fastly_routing_config_auto" "test" {
  name = "{{.ROUTING_CONFIG_NAME}}"

  path {
    path = "/"

    rule {
      action_type  = "service"
      action_value = {{.SERVICE_ID_REF}}

      condition {
        type     = "header"
        key      = "X-Beta"
        operator = "equals"
        value    = "true"
      }
    }

    rule {
      action_type  = "service"
      action_value = {{.SERVICE_ID_REF}}
    }
  }

  path {
    path = "/api/*"

    rule {
      action_type  = "service"
      action_value = {{.SERVICE_ID_REF}}
    }
  }
}
