logging_syslog {
  name              = "{{.LOGGING_SYSLOG_NAME}}"
  address           = "syslog-updated.example.com"
  message_type      = "loggly"
  processing_region = "eu"
  format_version    = 2
}
