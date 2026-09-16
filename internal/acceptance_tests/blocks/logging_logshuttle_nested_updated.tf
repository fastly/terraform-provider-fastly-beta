logging_logshuttle {
  name = "{{.LOGGING_LOGSHUTTLE_NAME}}"
  url  = "https://west.logplex.io/logs"
  authentication = {
    token = "updated-logshuttle-token"
  }
  processing_region = "eu"
  format_version    = 2
}
