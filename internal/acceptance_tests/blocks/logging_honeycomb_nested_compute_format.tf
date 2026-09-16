logging_honeycomb {
  name    = "{{.LOGGING_HONEYCOMB_NAME}}"
  dataset = "test-dataset"
  authentication = {
    token = "test-honeycomb-key"
  }
  format = "%h %l %u %t \"%r\" %>s %b"
}
