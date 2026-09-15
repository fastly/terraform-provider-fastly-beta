resource "fastly_service_logging_honeycomb" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_HONEYCOMB_NAME}}"
  dataset    = "updated-dataset"
  authentication = {
    token = "updated-honeycomb-key"
  }
  processing_region = "eu"
  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
}
