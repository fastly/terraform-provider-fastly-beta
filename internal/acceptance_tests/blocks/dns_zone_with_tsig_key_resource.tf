resource "fastly_tsig_key" "test" {
  name      = "{{.KEY_NAME}}"
  algorithm = "hmac-sha256"
  secret = {
    value = "{{.SECRET}}"
  }
}

resource "fastly_dns_zone" "test" {
  name        = "{{.ZONE_NAME}}"
  description = "{{.ZONE_DESCRIPTION}}"

  xfr_config_inbound {
    inbound_tsig_key_id = fastly_tsig_key.test.id

    primaries {
      address     = "{{.PRIMARY_ADDRESS}}"
      description = "{{.PRIMARY_DESCRIPTION}}"
    }
  }
}
