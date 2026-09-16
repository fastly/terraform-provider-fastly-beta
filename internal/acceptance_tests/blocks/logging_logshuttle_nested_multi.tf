logging_logshuttle {
  name = "{{.LOGGING_LOGSHUTTLE_NAME_1}}"
  url  = "https://east.logplex.io/logs"
  authentication = {
    token = "test-logshuttle-token"
  }
}

logging_logshuttle {
  name = "{{.LOGGING_LOGSHUTTLE_NAME_2}}"
  url  = "https://east.logplex.io/logs"
  authentication = {
    token = "test-logshuttle-token"
  }
}
