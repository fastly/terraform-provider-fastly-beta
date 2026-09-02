resource "fastly_tls_private_key" "test" {
  name = "{{.NAME}}"
  private_key = {
    pem = <<EOT
{{.KEY_PEM}}
EOT
  }
}
