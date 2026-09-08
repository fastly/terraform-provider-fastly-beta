---
page_title: "fastly_tls_certificate Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to get the ID of a TLS certificate, or to look up other details of an existing certificate.
---

# fastly_tls_certificate (Data Source)

Use this data source to get the ID of a TLS certificate, or to look up other details of an existing certificate.

The filters are applied using an **AND** boolean operator, so depending on the combination of filters, they may become mutually exclusive. `id` must not be specified in combination with any of the others.

If more or less than a single match is returned by the search, the read fails. Ensure that your search is specific enough to return a single result.

## Example Usage

```terraform
data "fastly_tls_certificate" "example" {
  name = "example.com"
}
```

## Schema

### Optional

- `domains` (Set of String) Domains that are listed in any certificates' Subject Alternative Names (SAN) list.
- `id` (String) Unique ID assigned to certificate by Fastly.
- `issued_to` (String) The hostname for which a certificate was issued.
- `issuer` (String) The certificate authority that issued the certificate.
- `name` (String) Human-readable name used to identify the certificate. Defaults to the certificate's Common Name or first Subject Alternative Name entry.

### Read-Only

- `created_at` (String) Timestamp (GMT) when the certificate was created.
- `replace` (Boolean) A recommendation from Fastly indicating the key associated with this certificate is in need of rotation.
- `serial_number` (String) A value assigned by the issuer that is unique to a certificate.
- `signature_algorithm` (String) The algorithm used to sign the certificate.
- `updated_at` (String) Timestamp (GMT) when the certificate was last updated.
