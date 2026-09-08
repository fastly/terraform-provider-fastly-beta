data "fastly_tls_configuration" "platform" {
  tls_service = "PLATFORM"
}

resource "fastly_tls_platform_certificate" "test" {
  certificate_body   = <<EOT
{{.CERTIFICATE_BODY}}
EOT
  intermediates_blob = <<EOT
{{.INTERMEDIATES_BLOB}}
EOT

  configuration_id     = data.fastly_tls_configuration.platform.id
  allow_untrusted_root = true
}

data "fastly_tls_platform_certificate_ids" "test" {
  depends_on = [fastly_tls_platform_certificate.test]
}
