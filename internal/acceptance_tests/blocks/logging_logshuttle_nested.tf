logging_logshuttle {
  name = "{{.LOGGING_LOGSHUTTLE_NAME}}"
  url  = "https://east.logplex.io/logs"
  authentication = {
    token = "test-logshuttle-token"
  }
}
