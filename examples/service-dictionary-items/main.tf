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
  name          = "example-service"
  force_destroy = true

  domain {
    name = "example.com"
  }

  dictionary {
    name = "application_configuration"
  }
}

# Manage Dictionary items with an explicit resource. Terraform only manages
# the keys declared below; any other items already in the Dictionary are
# left untouched.
resource "fastly_service_dictionary_items" "example" {
  service_id    = fastly_service_cdn_auto.example.id
  dictionary_id = fastly_service_cdn_auto.example.dictionary[0].dictionary_id

  items = {
    environment = "production"
    feature_x   = "enabled"
    api_version = "v2"
  }
}
