# Preserve unrelated Terraform configuration.
terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
    }
  }
}

variable "service_count" {
  type    = number
  default = 1
}

variable "service_comment" {
  type    = string
  default = ""
}

variable "origin_address" {
  type    = string
  default = "origin.example.com"
}

locals {
  service_name = "edge"
}

provider "fastly" {
  alias = "edge"
}

resource "fastly_service_vcl" "demo" {
  count    = var.service_count
  provider = fastly.edge
  name     = local.service_name
  comment  = var.service_comment

  backend {
    name    = "origin"
    address = var.origin_address
  }
}

output "service_name" {
  value = local.service_name
}
