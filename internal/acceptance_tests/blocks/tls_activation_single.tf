resource "fastly_tls_activation" "test" {
  certificate_id = {{.CERTIFICATE_ID_EXPR}}
  domain         = "{{.DOMAIN_NAME}}"
  depends_on     = [fastly_service_cdn_auto.test{{.EXTRA_DEPENDS_ON}}]
}
