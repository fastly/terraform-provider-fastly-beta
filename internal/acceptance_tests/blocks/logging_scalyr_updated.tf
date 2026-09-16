resource "fastly_service_logging_scalyr" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_SCALYR_NAME}}"
  authentication = {
    token = "updated-scalyr-token"
  }
  project_id         = "updated-project"
  region             = "EU"
  processing_region  = "eu"
  format             = "%h %l %u %t \"%r\" %>s %b"
  format_version     = 2
  placement          = "none"
}
