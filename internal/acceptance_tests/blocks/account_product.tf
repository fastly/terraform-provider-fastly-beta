resource "fastly_product_{{.PRODUCT}}" "test" {
  {{if .ENABLED}}enabled = {{.ENABLED}}{{end}}
}
