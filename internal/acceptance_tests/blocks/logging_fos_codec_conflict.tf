resource "fastly_service_logging_fos" "test" {
  service_id        = fastly_service_cdn.test.id
  version           = {{.SERVICE_VERSION}}
  name              = "{{.LOGGING_FOS_NAME}}"
  bucket_name       = "test-fos-bucket"
  region            = "us-east"
  compression_codec = "zstd"
  gzip_level        = 3
}
