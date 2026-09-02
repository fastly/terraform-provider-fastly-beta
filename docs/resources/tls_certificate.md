---
page_title: "fastly_tls_certificate Resource - fastly"
subcategory: ""
description: |-
  Uploads a custom TLS certificate. TLS certificates are versionless and independent of any service-version lifecycle.
---

# fastly_tls_certificate (Resource)

Uploads a custom TLS certificate. TLS certificates are versionless and independent of any service-version lifecycle.

The certificate's corresponding private key must already be uploaded to Fastly before the certificate can be created.

## Example Usage

```terraform
resource "fastly_tls_certificate" "example" {
  certificate_body = file("example.com.crt")
  name             = "example-cert"
}
```

## Updating certificates

There are three scenarios for updating a certificate:

1. The certificate is about to expire but the private key stays the same.
2. The certificate is about to expire but the private key is changing.
3. The domains on the certificate are changing.

In the first scenario you only need to update the `certificate_body` attribute of this resource. The other scenarios require a new private key and certificate to be created, done in multiple plan/apply steps to avoid downtime: create the new key/certificate pair alongside the currently active ones, update `fastly_tls_activation.certificate_id` to point at the new certificate, then delete the old key/certificate.

## Schema

### Required

- `certificate_body` (String) PEM-formatted certificate, optionally including any intermediary certificates.

### Optional

- `name` (String) Human-readable name used to identify the certificate. Defaults to the certificate's Common Name or first Subject Alternative Name entry.

### Read-Only

- `created_at` (String) Timestamp (GMT) when the certificate was created.
- `domains` (Set of String) All the domains (including wildcard domains) that are listed in the certificate's Subject Alternative Names (SAN) list.
- `id` (String) Alphanumeric string identifying a TLS certificate.
- `issued_to` (String) The hostname for which a certificate was issued.
- `issuer` (String) The certificate authority that issued the certificate.
- `replace` (Boolean) A recommendation from Fastly indicating the key associated with this certificate is in need of rotation.
- `serial_number` (String) A value assigned by the issuer that is unique to a certificate.
- `signature_algorithm` (String) The algorithm used to sign the certificate.
- `updated_at` (String) Timestamp (GMT) when the certificate was last updated.

## Import

A certificate can be imported using its Fastly certificate ID, e.g.

```shell
terraform import fastly_tls_certificate.example xxxxxxxx
```
