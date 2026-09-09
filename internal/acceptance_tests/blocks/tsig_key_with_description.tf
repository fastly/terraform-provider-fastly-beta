resource "fastly_tsig_key" "test" {
  name        = "{{.NAME}}"
  algorithm   = "{{.ALGORITHM}}"
  description = "{{.DESCRIPTION}}"
  secret = {
    value = "{{.SECRET}}"
  }
}
