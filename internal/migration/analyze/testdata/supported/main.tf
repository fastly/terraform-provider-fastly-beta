terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "~> 5.0"
    }
  }
}

variable "backend_host" {
  type    = string
  default = "origin.example.com"
}

module "service" {
  source       = "./modules/service"
  backend_host = var.backend_host
}

output "service_id" {
  value = module.service.service_id
}
