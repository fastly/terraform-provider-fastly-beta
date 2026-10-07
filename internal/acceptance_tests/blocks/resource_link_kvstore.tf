resource "fastly_kvstore" "store" {
  name = "{{.KVSTORE_NAME}}"
}

resource "fastly_service_resource_link" "test" {
  service_id  = fastly_service_compute.test.id
  version     = {{.SERVICE_VERSION}}
  name        = "{{.RESOURCE_LINK_NAME}}"
  resource_id = fastly_kvstore.store.id
}
