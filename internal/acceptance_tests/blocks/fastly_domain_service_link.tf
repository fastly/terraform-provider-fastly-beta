resource "fastly_domain" "test" {
  fqdn = "{{.DOMAIN_FQDN}}"
}

resource "fastly_domain_service_link" "test" {
  domain_id  = fastly_domain.test.id
  service_id = {{.SERVICE_ID_REF}}
}
