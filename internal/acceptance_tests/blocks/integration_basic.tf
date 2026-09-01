resource "fastly_integration" "test" {
  name        = "{{.NAME}}"
  description = "{{.DESCRIPTION}}"
  type        = "{{.TYPE}}"
  config      = {{.CONFIG}}
}
