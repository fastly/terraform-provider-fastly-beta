variable "activate" {
  type = bool
}

resource "fastly_service_vcl" "demo" {
  name     = "migration-demo"
  activate = var.activate
}
