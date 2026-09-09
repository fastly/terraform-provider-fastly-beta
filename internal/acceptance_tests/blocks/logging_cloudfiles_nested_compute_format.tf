logging_cloudfiles {
  name        = "{{.LOGGING_CLOUDFILES_NAME}}"
  bucket_name = "test-cloudfiles-bucket"
  authentication = {
    user       = "test-user"
    access_key = "test-access-key"
  }
  format = "%h %l %u %t \"%r\" %>s %b"
}
