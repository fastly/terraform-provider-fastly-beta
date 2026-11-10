resource "fastly_account_product_{{.PRODUCT}}" "test" {
  {{if .ENABLED}}enabled = {{.ENABLED}}{{end}}
}
