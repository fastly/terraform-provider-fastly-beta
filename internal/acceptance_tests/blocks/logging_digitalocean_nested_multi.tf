logging_digitalocean {
  name        = "{{.LOGGING_DIGITALOCEAN_NAME_1}}"
  bucket_name = "test-digitalocean-bucket"
  authentication = {
    access_key = "test-access-key"
    secret_key = "test-secret-key"
  }
}

logging_digitalocean {
  name        = "{{.LOGGING_DIGITALOCEAN_NAME_2}}"
  bucket_name = "test-digitalocean-bucket"
  authentication = {
    access_key = "test-access-key"
    secret_key = "test-secret-key"
  }
}
