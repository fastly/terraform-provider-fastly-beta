resource "fastly_tsig_key" "test" {
  name      = "{{.KEY_NAME}}"
  algorithm = "hmac-sha256"
  secret = {
    value = "{{.SECRET}}"
  }
}

resource "fastly_dns_zone" "test" {
  name        = "{{.ZONE_NAME}}"
  description = ""

  xfr_config_inbound {
    primaries {
      address     = "{{.PRIMARY_ADDRESS}}"
      description = "{{.PRIMARY_DESCRIPTION}}"
    }
  }
}
