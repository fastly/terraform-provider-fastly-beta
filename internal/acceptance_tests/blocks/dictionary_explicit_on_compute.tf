resource "fastly_service_dictionary" "test" {
  service_id    = fastly_service_compute.test.id
  version       = {{.SERVICE_VERSION}}
  name          = "{{.DICTIONARY_NAME}}"
  force_destroy = true
}
