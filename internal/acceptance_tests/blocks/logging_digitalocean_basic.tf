resource "fastly_service_logging_digitalocean" "test" {
  service_id  = fastly_service_cdn.test.id
  version     = {{.SERVICE_VERSION}}
  name        = "{{.LOGGING_DIGITALOCEAN_NAME}}"
  bucket_name = "test-digitalocean-bucket"
  authentication = {
    access_key = "test-access-key"
    secret_key = "test-secret-key"
  }
}
