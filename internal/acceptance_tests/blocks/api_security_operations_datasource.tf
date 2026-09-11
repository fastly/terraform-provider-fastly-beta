resource "fastly_api_security_operation" "example" {
  service_id = fastly_service_cdn.test.id
  method     = "{{.METHOD}}"
  domain     = "{{.DOMAIN}}"
  path       = "{{.PATH}}"
}

data "fastly_api_security_operations" "example" {
  service_id = fastly_service_cdn.test.id
  depends_on = [fastly_api_security_operation.example]
}
