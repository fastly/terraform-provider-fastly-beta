resource "fastly_routing_config_auto" "test" {
  name = "{{.ROUTING_CONFIG_NAME}}"

  path {
    path = "/z"

    rule {
      action_type  = "service"
      action_value = {{.SERVICE_ID_REF}}

      condition {
        type     = "header"
        key      = "{{.FIRST_HEADER}}"
        operator = "equals"
        value    = "true"
      }
    }

    rule {
      action_type  = "service"
      action_value = {{.SERVICE_ID_REF}}

      condition {
        type     = "header"
        key      = "{{.SECOND_HEADER}}"
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
    path = "/a"

    rule {
      action_type  = "service"
      action_value = {{.SERVICE_ID_REF}}
    }
  }
}
