resource "fastly_dns_zone" "test" {
  name        = "{{.ZONE_NAME}}"
  description = "{{.ZONE_DESCRIPTION}}"

  xfr_config_inbound {
    inbound_tsig_key_id = "{{.TSIG_KEY_ID}}"

    primaries {
      address     = "{{.PRIMARY_ADDRESS}}"
      description = "{{.PRIMARY_DESCRIPTION}}"
    }
  }
}
