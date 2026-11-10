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

# These products are enabled for the whole account, so the resources take no
# service_id. Destroying one disables the product for every service on the
# account. Each can be imported using the product's own identifier, for
# example: terraform import fastly_account_product_ai_accelerator.example ai_accelerator

resource "fastly_account_product_ai_accelerator" "example" {}

resource "fastly_account_product_ai_runtime_control" "example" {}

resource "fastly_account_product_domain_research" "example" {}

resource "fastly_account_product_object_storage" "example" {}
