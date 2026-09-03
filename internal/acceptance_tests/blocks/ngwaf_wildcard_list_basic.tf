resource "fastly_ngwaf_wildcard_list" "test" {
  name        = "{{.LIST_NAME}}"
  description = "{{.LIST_DESCRIPTION}}"

  entries = {{.LIST_ENTRIES}}
}
