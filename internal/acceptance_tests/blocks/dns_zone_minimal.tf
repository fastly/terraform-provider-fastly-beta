resource "fastly_dns_zone" "test" {
  name = "{{.ZONE_NAME}}"
}
