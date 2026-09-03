resource "fastly_tls_private_key" "{{.RESOURCE_NAME}}" {
  name = "{{.NAME}}"

  private_key = {
    pem = <<-EOT
{{.KEY_PEM}}
EOT
  }
}

resource "fastly_tls_certificate" "{{.RESOURCE_NAME}}" {
  name             = "{{.NAME}}"
  certificate_body = <<-EOT
{{.CERT_PEM}}
EOT
  depends_on       = [fastly_tls_private_key.{{.RESOURCE_NAME}}]
}
