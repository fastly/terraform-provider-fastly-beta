logging_digitalocean {
  name        = "{{.LOGGING_DIGITALOCEAN_NAME}}"
  bucket_name = "test-digitalocean-bucket"
  authentication = {
    access_key = "test-access-key"
    secret_key = "test-secret-key"
  }
}
