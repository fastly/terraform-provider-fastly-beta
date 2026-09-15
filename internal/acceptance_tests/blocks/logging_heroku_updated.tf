resource "fastly_service_logging_heroku" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_HEROKU_NAME}}"
  url        = "https://updated.herokuapp.com/log"
  authentication = {
    token = "updated-heroku-token"
  }
  processing_region = "eu"
  format            = "%h %l %u %t \"%r\" %>s %b"
  format_version    = 2
  placement         = "none"
}
