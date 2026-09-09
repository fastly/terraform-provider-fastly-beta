resource "fastly_tsig_key" "test" {
  name      = "{{.NAME}}"
  algorithm = "{{.ALGORITHM}}"
  secret = {
    value = "{{.SECRET}}"
  }
}
