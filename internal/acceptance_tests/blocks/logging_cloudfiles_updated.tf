resource "fastly_service_logging_cloudfiles" "test" {
  service_id  = fastly_service_cdn.test.id
  version     = {{.SERVICE_VERSION}}
  name        = "{{.LOGGING_CLOUDFILES_NAME}}"
  bucket_name = "test-cloudfiles-bucket"
  authentication = {
    user       = "test-user"
    access_key = "updated-access-key"
  }
  region            = "LON"
  processing_region = "eu"
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
