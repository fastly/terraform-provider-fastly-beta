resource "fastly_integration" "test" {
  name = "{{.NAME}}"
  type = "not-a-real-type"
  config = {
    "key" = "value"
  }
}
