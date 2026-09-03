---
page_title: "fastly_tls_subscription_validation Resource - fastly"
subcategory: ""
description: |-
  This resource represents a successful validation of a Fastly TLS Subscription in concert with other resources. Most commonly, this resource is used together with a resource for a DNS record and fastly_tls_subscription to request a DNS validated certificate, deploy the required validation records and wait for validation to complete.
  This resource implements a part of the validation workflow. It does not represent a real-world entity in Fastly, therefore changing or deleting this resource on its own has no immediate effect. Waits up to 45 minutes for the subscription to reach the issued state.
---

# fastly_tls_subscription_validation (Resource)

This resource represents a successful validation of a Fastly TLS Subscription in concert with other resources. Most commonly, this resource is used together with a resource for a DNS record and `fastly_tls_subscription` to request a DNS validated certificate, deploy the required validation records and wait for validation to complete.

This resource implements a part of the validation workflow. It does not represent a real-world entity in Fastly, therefore changing or deleting this resource on its own has no immediate effect. Waits up to 45 minutes for the subscription to reach the `issued` state.

## Example Usage

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "example-service"

  domain {
    name = "example.com"
  }

  backend {
    address = "127.0.0.1"
    name    = "localhost"
  }
}

resource "fastly_tls_subscription" "example" {
  domains               = [for domain in fastly_service_cdn_auto.example.domain : domain.name]
  certificate_authority = "lets-encrypt"
  force_destroy         = true

  depends_on = [fastly_service_cdn_auto.example]
}

# Create the DNS record(s) required to respond to the ACME domain ownership
# challenge with your DNS provider's own resources, fed from
# fastly_tls_subscription.example.managed_dns_challenges (or
# managed_http_challenges).

resource "fastly_tls_subscription_validation" "example" {
  subscription_id = fastly_tls_subscription.example.id

  # depends_on should include the DNS validation record resource(s) above,
  # so the challenge is in place before Fastly attempts to validate it.
}

# certificate_id is only populated once the subscription reaches the "issued"
# state, so resources referencing it are guaranteed to run after the
# certificate exists - unlike fastly_tls_subscription.example.certificate_id,
# which is empty until the certificate is issued asynchronously.
output "certificate_id" {
  value = fastly_tls_subscription_validation.example.certificate_id
}
```

## Schema

### Required

- `subscription_id` (String) The ID of the TLS Subscription that should be validated.

### Optional

- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))

### Read-Only

- `certificate_id` (String) The ID of the certificate issued for the validated subscription. Only populated once the subscription reaches the `issued` state. `fastly_tls_subscription` activates TLS on its domains automatically, so this attribute is informational only and should not be used to create a `fastly_tls_activation` for those domains.
- `id` (String) The ID of this resource. Matches `subscription_id` once the subscription has been validated.

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).
