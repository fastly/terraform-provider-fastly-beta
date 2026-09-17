logging_openstack {
  name        = "{{.LOGGING_OPENSTACK_NAME_1}}"
  bucket_name = "test-openstack-bucket"
  url         = "https://auth.storage.example.com/v1.0"
  authentication = {
    user       = "test-user"
    access_key = "test-access-key"
  }
}

logging_openstack {
  name        = "{{.LOGGING_OPENSTACK_NAME_2}}"
  bucket_name = "test-openstack-bucket"
  url         = "https://auth.storage.example.com/v1.0"
  authentication = {
    user       = "test-user"
    access_key = "test-access-key"
  }
}
