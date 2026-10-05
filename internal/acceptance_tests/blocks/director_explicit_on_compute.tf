resource "fastly_service_director" "test" {
  service_id = fastly_service_compute.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.DIRECTOR_NAME}}"
  backends   = ["origin1"]
}
