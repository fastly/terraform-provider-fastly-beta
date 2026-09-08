resource "fastly_configstore_items" "items" {
  store_id = fastly_configstore.store.id
  items    = {{.ITEMS}}
}
