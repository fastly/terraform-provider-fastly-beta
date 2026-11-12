resource "fastly_{{.PRODUCT}}" "test" {
  {{if .ENABLED}}enabled = {{.ENABLED}}{{end}}
}
