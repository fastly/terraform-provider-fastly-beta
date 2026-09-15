logging_heroku {
  name = "{{.LOGGING_HEROKU_NAME}}"
  url  = "https://example.herokuapp.com/log"
  authentication = {
    token = "test-heroku-token"
  }
  format = "%h %l %u %t \"%r\" %>s %b"
}
