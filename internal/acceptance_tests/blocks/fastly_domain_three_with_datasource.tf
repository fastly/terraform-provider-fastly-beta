resource "fastly_domain" "domain1" {
  fqdn = "{{.DOMAIN_FQDN_1}}"
}

resource "fastly_domain" "domain2" {
  fqdn = "{{.DOMAIN_FQDN_2}}"
}

resource "fastly_domain" "domain3" {
  fqdn = "{{.DOMAIN_FQDN_3}}"
}

data "fastly_domains" "example" {
  depends_on = [
    fastly_domain.domain1,
    fastly_domain.domain2,
    fastly_domain.domain3,
  ]
}
