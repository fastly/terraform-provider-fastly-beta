terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly-beta"
    }
  }
}

provider "fastly" {
  # API token set via FASTLY_API_TOKEN environment variable
}

# Destroying one of these resources disables the product. Import using the
# product ID, for example: terraform import fastly_ai_accelerator.example ai_accelerator

resource "fastly_ai_accelerator" "example" {}

resource "fastly_ai_runtime_control" "example" {}

resource "fastly_domain_research" "example" {}

resource "fastly_object_storage" "example" {}
