resource "fastly_ngwaf_ip_list" "test" {
  name        = "{{.LIST_NAME}}"
  description = "{{.LIST_DESCRIPTION}}"

  entries = {{.LIST_ENTRIES}}
}
