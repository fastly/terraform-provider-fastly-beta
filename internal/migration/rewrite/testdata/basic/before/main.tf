# Preserve this header and all unmodified formatting.
locals {
  service_name = "edge"
}

resource "fastly_service_vcl" "demo" {
  count = var.count
  name  = local.service_name
  provider = fastly.edge

  backend {
    name    = "origin"
    address = var.origin
  }

  # Keep this comment with the attribute.
  version_note = "${var.version}" # preserve trailing comment
}

output "service_id" {
  value = fastly_service_vcl.demo[0].id
}
