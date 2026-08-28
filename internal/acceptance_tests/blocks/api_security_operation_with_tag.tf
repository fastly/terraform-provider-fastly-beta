resource "fastly_api_security_operation_tag" "tag" {
  service_id = fastly_service_cdn.test.id
  name       = "{{.TAG_NAME}}"
}

resource "fastly_api_security_operation" "example" {
  service_id = fastly_service_cdn.test.id
  method     = "{{.METHOD}}"
  domain     = "{{.DOMAIN}}"
  path       = "{{.PATH}}"
  {{if .DESCRIPTION}}description = "{{.DESCRIPTION}}"{{end}}
  tag_ids = [fastly_api_security_operation_tag.tag.tag_id]
}
