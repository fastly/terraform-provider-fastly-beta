resource "fastly_domain" "test" {
  fqdn = "{{.DOMAIN_FQDN}}"
}
