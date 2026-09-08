resource "fastly_ngwaf_string_list" "test" {
  name        = "{{.LIST_NAME}}"
  description = "{{.LIST_DESCRIPTION_UPDATED}}"

  entries = {{.LIST_ENTRIES_UPDATED}}
}
