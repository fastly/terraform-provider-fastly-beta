terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly-beta"
    }
  }
}

provider "fastly" {
  # API token set via FASTLY_API_TOKEN environment variable
}

resource "fastly_service_compute" "app" {
  name    = "example-compute-service"
  comment = "Managed by Terraform"
}

resource "fastly_kvstore" "store" {
  name = "example-kv-store"
}

# Makes the KV Store available to Wasm code under the alias "store".
resource "fastly_service_resource_link" "store" {
  service_id  = fastly_service_compute.app.id
  version     = 1
  name        = "store"
  resource_id = fastly_kvstore.store.id
}
