resource "fastly_secretstore" "store" {
  name = "{{.SECRETSTORE_NAME}}"
}
