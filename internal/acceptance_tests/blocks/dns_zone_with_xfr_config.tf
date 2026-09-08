resource "fastly_dns_zone" "test" {
  name        = "{{.ZONE_NAME}}"
  description = "{{.ZONE_DESCRIPTION}}"

  xfr_config_inbound {
    primaries {
      address     = "{{.PRIMARY_ADDRESS}}"
      description = "{{.PRIMARY_DESCRIPTION}}"
    }
  }
}
