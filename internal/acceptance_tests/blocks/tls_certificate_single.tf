resource "fastly_tls_certificate" "test" {
  certificate_body = <<-EOT
{{.CERTIFICATE_BODY}}
EOT
  {{if .NAME}}name = "{{.NAME}}"{{end}}
  {{if .DEPENDS_ON_PRIVATE_KEY}}depends_on = [fastly_tls_private_key.test]{{end}}
}
