resource "fastly_service_dictionary" "test" {
  service_id    = fastly_service_cdn.test.id
  version       = {{.SERVICE_VERSION}}
  name          = "{{.DICTIONARY_NAME}}"
  write_only    = true
  force_destroy = true
}
