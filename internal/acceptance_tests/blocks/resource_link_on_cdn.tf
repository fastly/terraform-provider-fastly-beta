resource "fastly_service_resource_link" "test" {
  service_id  = fastly_service_cdn.test.id
  version     = {{.SERVICE_VERSION}}
  name        = "{{.RESOURCE_LINK_NAME}}"
  resource_id = "unused-resource-id"
}
