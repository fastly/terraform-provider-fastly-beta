logging_heroku {
  name = "{{.LOGGING_HEROKU_NAME}}"
  url  = "https://example.herokuapp.com/log"
  authentication = {
    token = "test-heroku-token"
  }
  placement = "none"
}
