resource "fastly_service_logging_fos" "test" {
  service_id        = fastly_service_cdn.test.id
  version           = {{.SERVICE_VERSION}}
  name              = "{{.LOGGING_FOS_NAME}}"
  bucket_name       = "test-fos-bucket"
  region            = "eu-central"
  path              = "logs/"
  period            = 600
  compression_codec = "zstd"
  file_max_bytes    = 2097152
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
