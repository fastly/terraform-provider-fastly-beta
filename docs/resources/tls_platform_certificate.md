---
page_title: "fastly_tls_platform_certificate Resource - fastly"
subcategory: ""
description: |-
  Uploads a TLS certificate to the Fastly Platform TLS service.
---

# fastly_tls_platform_certificate (Resource)

Uploads a TLS certificate to the Fastly Platform TLS service.

-> Each certificate's corresponding private key must be uploaded to Fastly's TLS Private Keys store
_before_ the certificate can be uploaded. In Terraform this can be achieved with
[`depends_on`](https://developer.hashicorp.com/terraform/language/meta-arguments/depends_on).

## Example Usage

Basic usage with a self-signed CA:

```terraform
resource "tls_private_key" "ca_key" {
  algorithm = "RSA"
}

resource "tls_private_key" "key" {
  algorithm = "RSA"
}

resource "tls_self_signed_cert" "ca" {
  private_key_pem = tls_private_key.ca_key.private_key_pem

  subject {
    common_name = "Example CA"
  }

  is_ca_certificate     = true
  validity_period_hours = 360

  allowed_uses = [
    "cert_signing",
    "server_auth",
  ]
}

resource "tls_cert_request" "example" {
  private_key_pem = tls_private_key.key.private_key_pem

  subject {
    common_name = "example.com"
  }

  dns_names = ["example.com", "www.example.com"]
}

resource "tls_locally_signed_cert" "cert" {
  cert_request_pem   = tls_cert_request.example.cert_request_pem
  ca_private_key_pem = tls_private_key.ca_key.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.ca.cert_pem

  validity_period_hours = 360

  allowed_uses = [
    "cert_signing",
    "server_auth",
  ]
}

data "fastly_tls_configuration" "config" {
  tls_service = "PLATFORM"
}

resource "fastly_tls_platform_certificate" "cert" {
  certificate_body   = tls_locally_signed_cert.cert.cert_pem
  intermediates_blob = tls_self_signed_cert.ca.cert_pem

  configuration_id     = data.fastly_tls_configuration.config.id
  allow_untrusted_root = true
}
```

## Schema

### Required

- `certificate_body` (String) PEM-formatted certificate.
- `configuration_id` (String) ID of the TLS configuration to be used to terminate TLS traffic. Changing this attribute will delete and recreate the certificate.
- `intermediates_blob` (String) PEM-formatted certificate chain from the `certificate_body` to its root.

### Optional

- `allow_untrusted_root` (Boolean) Disable checking whether the root of the certificate chain is trusted. Useful for development purposes to allow use of self-signed CAs. Defaults to `false`.

### Read-Only

- `created_at` (String) Timestamp (GMT) when the certificate was created.
- `domains` (Set of String) All the domains (including wildcard domains) that are listed in any certificate's Subject Alternative Names (SAN) list.
- `id` (String) The unique ID assigned to the certificate by Fastly.
- `not_after` (String) Timestamp (GMT) when the certificate will expire. Must be in the future for the certificate to terminate TLS traffic.
- `not_before` (String) Timestamp (GMT) when the certificate will become valid. Must be in the past for the certificate to terminate TLS traffic.
- `replace` (Boolean) A recommendation from Fastly indicating the key associated with this certificate is in need of rotation.
- `updated_at` (String) Timestamp (GMT) when the certificate was last updated.

## Import

A certificate can be imported using its Fastly certificate ID, e.g.

```shell
terraform import fastly_tls_platform_certificate.example xxxxxxxxxxx
```

Import does not recover `certificate_body`, `intermediates_blob`, or `allow_untrusted_root` - the
Fastly API never returns them for an existing certificate, so they are left unset in state after
import. Applying an unmodified configuration afterward re-sends the values already in that
configuration.
