logging_papertrail {
  name    = "{{.LOGGING_PAPERTRAIL_NAME}}"
  address = "logs.papertrailapp.com"
  port    = 12345
  format  = "%h %l %u %t \"%r\" %>s %b"
}
