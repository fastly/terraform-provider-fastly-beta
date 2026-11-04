resource "fastly_service_vcl" "demo" {
  name = "migration-demo"

  logging_logentries {
    name  = "logs"
    token = "secret"
  }
}
