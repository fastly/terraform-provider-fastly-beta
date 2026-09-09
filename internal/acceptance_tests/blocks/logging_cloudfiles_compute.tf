resource "fastly_service_logging_cloudfiles" "test" {
  service_id  = fastly_service_compute.test.id
  version     = {{.SERVICE_VERSION}}
  name        = "{{.LOGGING_CLOUDFILES_NAME}}"
  bucket_name = "test-cloudfiles-bucket"
  authentication = {
    user       = "test-user"
    access_key = "test-access-key"
  }
}
