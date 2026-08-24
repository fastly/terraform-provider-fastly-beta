terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
    }
  }
}

provider "fastly" {
  # API token set via FASTLY_API_TOKEN environment variable
}

resource "fastly_service_cdn_auto" "example" {
  name = "example-service"

  domain {
    name = "example.com"
  }

  backend {
    name    = "example-backend"
    address = "example.com"
    port    = 443
    shield  = "amsterdam-nl" # required for image_optimizer
  }

  force_destroy = true
}

# Each product is its own resource: creating it enables the product on
# service_id, destroying it disables the product. There's no separate
# "enabled" attribute to toggle.

resource "fastly_service_product_brotli_compression" "example" {
  service_id = fastly_service_cdn_auto.example.id
}

resource "fastly_service_product_image_optimizer" "example" {
  service_id = fastly_service_cdn_auto.example.id
}

resource "fastly_service_product_domain_inspector" "example" {
  service_id = fastly_service_cdn_auto.example.id
}

resource "fastly_service_product_websockets" "example" {
  service_id = fastly_service_cdn_auto.example.id
}

resource "fastly_service_product_ddos_protection" "example" {
  service_id = fastly_service_cdn_auto.example.id
  mode       = "log"
}

# Next-Gen WAF is enabled by linking a workspace_id to the service. The
# workspace itself is a versionless resource managed independently of any
# service or service version.

resource "fastly_ngwaf_workspace" "example" {
  name        = "example-workspace"
  description = "Managed by Terraform"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_service_product_ngwaf" "example" {
  service_id   = fastly_service_cdn_auto.example.id
  workspace_id = fastly_ngwaf_workspace.example.id
}

# Applying the same product to multiple services with for_each:

variable "service_ids" {
  type    = set(string)
  default = []
}

resource "fastly_service_product_domain_inspector" "fleet" {
  for_each = var.service_ids

  service_id = each.value
}
