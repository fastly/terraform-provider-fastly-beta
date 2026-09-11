data "fastly_api_security_discovered_operations" "example" {
  service_id = fastly_service_cdn.test.id
}
