resource "fastly_domain" "test" {
  fqdn = "{{.DOMAIN_FQDN}}"
}

resource "fastly_routing_config_domain_link" "test" {
  domain_id         = fastly_domain.test.id
  routing_config_id = fastly_routing_config_auto.test.id
}
