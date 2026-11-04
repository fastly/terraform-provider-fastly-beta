# Preserve this header and all unmodified formatting.
terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
    }
  }
}

provider "fastly" {
  alias = "edge"
}

variable "service_count" {
  type    = number
  default = 1
}

variable "origin" {
  type = string
}

variable "description" {
  type = string
}

locals {
  service_name = "edge"
}

resource "fastly_service_vcl" "demo" {
  count = var.service_count
  name  = local.service_name
  provider = fastly.edge

  backend {
    name    = "origin"
    address = var.origin
  }

  # Keep this comment with the service description.
  comment = "${var.description}" # preserve trailing comment
}

output "service_name" {
  value = local.service_name
}
