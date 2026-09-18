resource "fastly_service_logging_openstack" "test" {
  service_id  = fastly_service_cdn.test.id
  version     = {{.SERVICE_VERSION}}
  name        = "{{.LOGGING_OPENSTACK_NAME}}"
  bucket_name = "test-openstack-bucket"
  url         = "https://auth.storage.example.com/v1.0"
  authentication = {
    user       = "test-user"
    access_key = "test-access-key"
  }
  format = ""
}
