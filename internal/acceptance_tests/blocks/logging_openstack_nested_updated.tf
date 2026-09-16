logging_openstack {
  name        = "{{.LOGGING_OPENSTACK_NAME}}"
  bucket_name = "test-openstack-bucket"
  url         = "https://auth.storage.example.com/v1.0"
  authentication = {
    user       = "test-user"
    access_key = "updated-access-key"
  }
  processing_region = "eu"
  format_version    = 2
}
