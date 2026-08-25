resource "fastly_api_security_operation_tag" "example" {
  service_id = fastly_service_cdn.test.id
  name       = "{{.TAG_NAME}}"
  {{if .DESCRIPTION}}description = "{{.DESCRIPTION}}"{{end}}
}
