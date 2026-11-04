variable "backend_host" {
  type = string
}

resource "fastly_service_vcl" "demo" {
  name            = "migration-demo"
  activate        = true
  stage           = false
  version_comment = ""

  backend {
    name    = "origin"
    address = var.backend_host
  }

  product_enablement {
    domain_inspector = true
    websockets       = false
  }
}

output "service_id" {
  value = fastly_service_vcl.demo.id
}
