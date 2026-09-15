logging_heroku {
  name = "{{.LOGGING_HEROKU_NAME}}"
  url  = "https://updated.herokuapp.com/log"
  authentication = {
    token = "updated-heroku-token"
  }
  processing_region = "eu"
  format_version    = 2
}
