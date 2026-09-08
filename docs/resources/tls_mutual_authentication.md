---
page_title: "fastly_tls_mutual_authentication Resource - fastly"
subcategory: ""
description: |-
  Enables client-to-server mutual TLS. Mutual authentications are versionless and independent of any service-version lifecycle.
---

# fastly_tls_mutual_authentication (Resource)

Enables client-to-server mutual TLS. Mutual authentications are versionless and independent of any service-version lifecycle.

Mutual TLS can be added to an existing [`fastly_tls_activation`](tls_activation.md). You must already have active server-side TLS using either a custom certificate or an enabled Fastly-managed subscription before mutual TLS can be enabled.

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

resource "fastly_tls_certificate" "example" {
  certificate_body = file("example.com.crt")
  name             = "example-cert"
}

resource "fastly_tls_activation" "example" {
  certificate_id = fastly_tls_certificate.example.id
  domain         = "example.com"
  depends_on     = [fastly_service_cdn_auto.example]
}

resource "fastly_tls_mutual_authentication" "example" {
  activation_ids = [fastly_tls_activation.example.id]
  cert_bundle    = file("client-ca-bundle.crt")
  enforced       = true
}
```

## Schema

### Required

- `cert_bundle` (String) One or more certificates. Enter each individual certificate blob on a new line. Must be PEM-formatted.

### Optional

- `activation_ids` (Set of String) List of TLS Activation IDs
- `enforced` (Boolean) Determines whether Mutual TLS will fail closed (enforced) or fail open. A true value will require a successful Mutual TLS handshake for the connection to continue and will fail closed if unsuccessful. A false value will fail open and allow the connection to proceed (if this attribute is not set we default to `false`).
- `include` (String) A comma-separated list used by the Terraform provider during a state refresh to return more data related to your mutual authentication from the Fastly API (permitted values: `tls_activations`).
- `name` (String) A custom name for your mutual authentication. If name is not supplied we will auto-generate one.

### Read-Only

- `created_at` (String) Date and time in ISO 8601 format.
- `id` (String) Alphanumeric string identifying a mutual authentication.
- `tls_activations` (List of String) List of alphanumeric strings identifying TLS activations.
- `updated_at` (String) Date and time in ISO 8601 format.

## Import

A TLS mutual authentication can be imported using its ID, e.g.

```shell
terraform import fastly_tls_mutual_authentication.example xxxxxxxx
```
