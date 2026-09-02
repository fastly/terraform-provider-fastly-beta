resource "fastly_tls_mutual_authentication" "test" {
  cert_bundle = <<-EOT
{{.CERT_BUNDLE}}
EOT
  {{if .ENFORCED}}enforced = {{.ENFORCED}}{{end}}
  {{if .NAME}}name = "{{.NAME}}"{{end}}
}
