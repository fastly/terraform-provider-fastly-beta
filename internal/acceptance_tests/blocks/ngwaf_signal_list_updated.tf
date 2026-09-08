resource "fastly_ngwaf_signal_list" "test" {
  name        = "{{.LIST_NAME}}"
  description = "{{.LIST_DESCRIPTION_UPDATED}}"

  entries = {{.LIST_ENTRIES_UPDATED}}
}
