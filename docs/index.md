---
page_title: "fastly Provider"
description: |-
---

# fastly Provider

-> **Note:** The `-beta` suffix shown in this provider's name and navigation refers only to its Registry distribution during the beta program. Resource and data source type names are unaffected and keep their standard `fastly_` prefix (e.g. `fastly_service_cdn_auto`), with no `-beta` in the name.

## Example Usage

```terraform
# Terraform 0.13+ requires providers to be declared in a "required_providers" block
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly-beta"
      version = ">= 0.1.3"
    }
  }
}

# Configure the Fastly Provider
provider "fastly" {
  api_token = "test"
}

# Create a Service
resource "fastly_service_cdn_auto" "myservice" {
  name = "myawesometestservice"

  backend {
    name    = "backend"
    address = "backend.example.com"
  }
}

# Domains are versionless and attach to the service by ID
resource "fastly_domain" "myservice" {
  fqdn       = "www.example.com"
  service_id = fastly_service_cdn_auto.myservice.id
}
```

## Schema

### Optional

- `api_token` (String, Sensitive) The Fastly API token. Can also be set via the FASTLY_API_TOKEN environment variable.
