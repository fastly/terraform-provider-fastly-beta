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

resource "fastly_object_storage_access_keys" "example" {
  description = "access key for my application"
  permission  = "read-write-objects"
  buckets     = ["bucket1", "bucket2"]
}

output "access_key_id" {
  value = fastly_object_storage_access_keys.example.access_key_id
}

output "secret_key" {
  value     = fastly_object_storage_access_keys.example.authentication.secret_key
  sensitive = true
}
