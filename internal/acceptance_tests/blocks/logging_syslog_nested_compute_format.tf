logging_syslog {
  name    = "{{.LOGGING_SYSLOG_NAME}}"
  address = "syslog.example.com"
  format  = "%h %l %u %t \"%r\" %>s %b"
}
