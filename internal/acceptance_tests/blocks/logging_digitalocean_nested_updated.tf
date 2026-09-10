logging_digitalocean {
  name        = "{{.LOGGING_DIGITALOCEAN_NAME}}"
  bucket_name = "test-digitalocean-bucket"
  authentication = {
    access_key = "updated-access-key"
    secret_key = "updated-secret-key"
  }
  domain            = "sfo2.digitaloceanspaces.com"
  processing_region = "eu"
  format_version    = 2
}
