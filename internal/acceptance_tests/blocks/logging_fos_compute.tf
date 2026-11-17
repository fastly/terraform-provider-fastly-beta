resource "fastly_service_logging_fos" "test" {
  service_id  = fastly_service_compute.test.id
  version     = {{.SERVICE_VERSION}}
  name        = "{{.LOGGING_FOS_NAME}}"
  bucket_name = "test-fos-bucket"
  region      = "us-east"
}
