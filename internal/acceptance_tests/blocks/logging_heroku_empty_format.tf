resource "fastly_service_logging_heroku" "test" {
  service_id = fastly_service_cdn.test.id
  version    = {{.SERVICE_VERSION}}
  name       = "{{.LOGGING_HEROKU_NAME}}"
  url        = "https://example.herokuapp.com/log"
  authentication = {
    token = "test-heroku-token"
  }
  format = ""
}
