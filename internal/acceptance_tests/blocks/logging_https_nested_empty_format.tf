logging_https {
  name         = "{{.LOGGING_HTTPS_NAME}}"
  url          = "https://https.example.com/logs"
  message_type = "blank"
  format       = ""
  content_type = "text/plain"
  method       = "POST"
  placement    = "none"
}
