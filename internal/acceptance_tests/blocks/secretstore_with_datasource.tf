resource "fastly_secretstore" "store" {
  name = "{{.SECRETSTORE_NAME}}"
}

data "fastly_secretstores" "example" {
  depends_on = [fastly_secretstore.store]
}
