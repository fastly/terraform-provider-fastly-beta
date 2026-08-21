resource "fastly_configstore" "store" {
  name = "{{.CONFIGSTORE_NAME}}"
}

data "fastly_configstores" "example" {
  depends_on = [fastly_configstore.store]
}
