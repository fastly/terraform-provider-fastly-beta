logging_cloudfiles {
  name        = "{{.LOGGING_CLOUDFILES_NAME}}"
  bucket_name = "test-cloudfiles-bucket"
  authentication = {
    user       = "test-user"
    access_key = "updated-access-key"
  }
  region            = "LON"
  processing_region = "eu"
  format_version    = 2
}
