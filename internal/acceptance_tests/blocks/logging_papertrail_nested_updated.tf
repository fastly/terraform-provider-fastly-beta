logging_papertrail {
  name              = "{{.LOGGING_PAPERTRAIL_NAME}}"
  address           = "logs2.papertrailapp.com"
  port              = 54321
  processing_region = "eu"
  format_version    = 2
}
