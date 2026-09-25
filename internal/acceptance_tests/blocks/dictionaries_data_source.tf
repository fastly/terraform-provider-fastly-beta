data "fastly_dictionaries" "example" {
  depends_on      = [fastly_service_cdn_auto.test]
  service_id      = fastly_service_cdn_auto.test.id
  service_version = fastly_service_cdn_auto.test.active_version
}
