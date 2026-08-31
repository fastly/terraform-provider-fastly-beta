---
page_title: "fastly_domain Resource - fastly"
subcategory: ""
description: |-
  Provides a Fastly Domain.
---

# fastly_domain (Resource)

Provides a Fastly Domain.

This is the root, versionless domain resource: it is not tied to a service version and is
managed independently of any `fastly_service_cdn` or `fastly_service_compute` resource. To
attach a domain to a service, use `fastly_domain_service_link`.

## Example Usage

```terraform
resource "fastly_domain" "example" {
  fqdn        = "www.example.com"
  description = "Managed by Terraform"
}
```

## Schema

### Required

- `fqdn` (String) The fully-qualified domain name for your domain (e.g. `www.example.com`). Can be created, but not updated.

### Optional

- `description` (String) The description for your domain.
- `service_id` (String) The service_id associated with your domain or null if there is no association. Computed so that a link created via `fastly_domain_service_link` is picked up on refresh instead of being planned away.

### Read-Only

- `id` (String) The Domain Identifier (UUID).

## Import

Fastly Domains can be imported using their Domain ID, e.g.

```shell
terraform import fastly_domain.example <domain_id>
```
