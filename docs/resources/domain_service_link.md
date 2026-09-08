---
page_title: "fastly_domain_service_link Resource - fastly"
subcategory: ""
description: |-
  Links a versionless fastly_domain to a service.
---

# fastly_domain_service_link (Resource)

Links a versionless `fastly_domain` to a service, independent of managing the domain
itself.

`service_id` can also be set directly on `fastly_domain`. Managing the same domain's
link with both resources at once is redundant - pick one per domain, typically
`fastly_domain_service_link` when the domain and the service are owned by separate
Terraform configurations.

## Example Usage

```terraform
resource "fastly_service_cdn" "example" {
  name    = "my_vcl_service"
  version = 1

  force_destroy = true
}

resource "fastly_domain" "example" {
  fqdn = "www.example.com"
}

resource "fastly_domain_service_link" "example" {
  domain_id  = fastly_domain.example.id
  service_id = fastly_service_cdn.example.id
}
```

## Schema

### Required

- `domain_id` (String) The Domain Identifier of the versionless domain being linked (UUID).
- `service_id` (String) The service_id associated with your domain.

### Read-Only

- `id` (String) The ID of this resource (identical to `domain_id`).

## Import

Fastly Domain Service Links can be imported using the linked Domain ID, e.g.

```shell
terraform import fastly_domain_service_link.example <domain_id>
```
