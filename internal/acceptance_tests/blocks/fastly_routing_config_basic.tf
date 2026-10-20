resource "fastly_routing_config" "test" {
  name = "{{.ROUTING_CONFIG_NAME}}"

  path {
    path = "/"

    rule {
      action_type  = "service"
      action_value = {{.SERVICE_ID_REF}}
    }
  }
}
