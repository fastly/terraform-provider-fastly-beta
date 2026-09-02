resource "fastly_ngwaf_signal" "test" {
  applies_to  = ["*"]
  name        = "{{.SIGNAL_NAME}}"
  description = "Terraform account signal lifecycle updated"
}
