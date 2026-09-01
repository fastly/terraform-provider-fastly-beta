resource "fastly_domain" "test" {
  fqdn        = "{{.DOMAIN_FQDN}}"
  description = "{{.DOMAIN_DESCRIPTION}}"
}
