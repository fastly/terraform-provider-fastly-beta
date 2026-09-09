resource "fastly_tsig_key" "key1" {
  name      = "{{.NAME_1}}"
  algorithm = "hmac-sha256"
  secret = {
    value = "dGVzdHNlY3JldA=="
  }
}

resource "fastly_tsig_key" "key2" {
  name      = "{{.NAME_2}}"
  algorithm = "hmac-sha256"
  secret = {
    value = "dGVzdHNlY3JldA=="
  }
}

data "fastly_tsig_keys" "example" {
  depends_on = [
    fastly_tsig_key.key1,
    fastly_tsig_key.key2,
  ]
}
