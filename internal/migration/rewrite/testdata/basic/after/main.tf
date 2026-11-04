# Preserve this header and all unmodified formatting.
locals {
  service_name = "edge"
}

resource "fastly_service_cdn_auto" "demo" {
  count = var.count
  name  = local.service_name
  provider = fastly.edge

  origin {
    name    = "origin"
    hostname = var.origin
  }

  settings {
    # Keep this comment with the attribute.
    note = "${var.version}" # preserve trailing comment
  }
}

output "service_id" {
  value = fastly_service_vcl.demo[0].id
}
