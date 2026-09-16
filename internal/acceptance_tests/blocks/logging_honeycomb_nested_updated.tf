logging_honeycomb {
  name    = "{{.LOGGING_HONEYCOMB_NAME}}"
  dataset = "updated-dataset"
  authentication = {
    token = "updated-honeycomb-key"
  }
  processing_region = "eu"
  format_version     = 2
}
