resource "fastly_service_dictionary_items" "items" {
  service_id    = {{.SERVICE_ID_REF}}
  dictionary_id = {{.DICTIONARY_ID_REF}}
  items         = {{.ITEMS}}
}
