resource "fastly_api_security_operation_tag" "example" {
  service_id = fastly_service_cdn.test.id
  name       = "{{.TAG_NAME}}"
}

data "fastly_api_security_operation_tags" "example" {
  service_id = fastly_service_cdn.test.id
  depends_on = [fastly_api_security_operation_tag.example]
}
