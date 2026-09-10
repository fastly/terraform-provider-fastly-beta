resource "fastly_service_logging_digitalocean" "test" {
  service_id  = fastly_service_cdn.test.id
  version     = {{.SERVICE_VERSION}}
  name        = "{{.LOGGING_DIGITALOCEAN_NAME}}"
  bucket_name = "test-digitalocean-bucket"
  authentication = {
    access_key = "updated-access-key"
    secret_key = "updated-secret-key"
  }
  domain            = "sfo2.digitaloceanspaces.com"
  processing_region = "eu"
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
