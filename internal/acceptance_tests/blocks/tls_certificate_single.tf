resource "fastly_tls_certificate" "test" {
  certificate_body = <<-EOT
{{.CERTIFICATE_BODY}}
EOT
  {{if .NAME}}name = "{{.NAME}}"{{end}}
}
