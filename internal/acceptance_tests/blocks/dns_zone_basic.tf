resource "fastly_dns_zone" "test" {
  name        = "{{.ZONE_NAME}}"
  description = "{{.ZONE_DESCRIPTION}}"
}
