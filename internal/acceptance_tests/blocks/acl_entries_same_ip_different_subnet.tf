resource "fastly_service_cdn_acl_entries" "test" {
  service_id = fastly_service_cdn.test.id
  acl_id     = fastly_service_cdn_acl.test.acl_id

  entry {
    ip     = "10.0.0.1"
    subnet = 24
  }

  entry {
    ip     = "10.0.0.1"
    subnet = 32
  }
}
