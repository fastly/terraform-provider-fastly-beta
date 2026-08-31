resource "fastly_dns_zone" "zone1" {
  name = "{{.ZONE_NAME_1}}"
}

resource "fastly_dns_zone" "zone2" {
  name = "{{.ZONE_NAME_2}}"
}

resource "fastly_dns_zone" "zone3" {
  name = "{{.ZONE_NAME_3}}"
}

data "fastly_dns_zones" "example" {
  depends_on = [
    fastly_dns_zone.zone1,
    fastly_dns_zone.zone2,
    fastly_dns_zone.zone3,
  ]
}
