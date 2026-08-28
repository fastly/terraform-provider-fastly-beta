resource "fastly_api_security_operation" "example" {
  service_id = fastly_service_cdn.test.id
  method     = "{{.METHOD}}"
  domain     = "{{.DOMAIN}}"
  path       = "{{.PATH}}"
  {{if .DESCRIPTION}}description = "{{.DESCRIPTION}}"{{end}}
}
