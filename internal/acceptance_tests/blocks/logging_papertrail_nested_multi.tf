logging_papertrail {
  name    = "{{.LOGGING_PAPERTRAIL_NAME_1}}"
  address = "logs.papertrailapp.com"
  port    = 12345
}

logging_papertrail {
  name    = "{{.LOGGING_PAPERTRAIL_NAME_2}}"
  address = "logs.papertrailapp.com"
  port    = 12346
}
