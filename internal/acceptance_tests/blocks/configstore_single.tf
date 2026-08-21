resource "fastly_configstore" "store" {
  name = "{{.CONFIGSTORE_NAME}}"
}
