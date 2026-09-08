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

data "fastly_tls_configuration_ids" "all" {}

output "tls_configuration_ids" {
  value = data.fastly_tls_configuration_ids.all.ids
}

data "fastly_tls_configuration" "default" {
  default = true
}

output "default_tls_configuration" {
  value = data.fastly_tls_configuration.default
}
