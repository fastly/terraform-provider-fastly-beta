resource "fastly_tls_activation" "test" {
  certificate_id = "{{.CERTIFICATE_ID}}"
  domain         = "{{.DOMAIN_NAME}}"
  depends_on     = [fastly_service_cdn_auto.test]
}
