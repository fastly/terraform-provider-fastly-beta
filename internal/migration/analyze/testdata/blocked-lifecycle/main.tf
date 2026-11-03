resource "fastly_service_vcl" "demo" {
  name            = "migration-demo"
  activate        = false
  stage           = true
  version_comment = "keep this comment"
}
