logging_heroku {
  name = "{{.LOGGING_HEROKU_NAME_1}}"
  url  = "https://example.herokuapp.com/log"
  authentication = {
    token = "test-heroku-token"
  }
}

logging_heroku {
  name = "{{.LOGGING_HEROKU_NAME_2}}"
  url  = "https://example.herokuapp.com/log"
  authentication = {
    token = "test-heroku-token"
  }
}
