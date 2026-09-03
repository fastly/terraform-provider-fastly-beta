---
page_title: "fastly_tls_private_key Data Source - fastly"
subcategory: ""
description: |-
  Use this data source to get the ID of a TLS private key, or to look up its metadata by name, key type, key length, or public key SHA1. The private key material itself is never returned once set.
---

# fastly_tls_private_key (Data Source)

Use this data source to get the ID of a TLS private key, or to look up its metadata by name, key type, key length, or public key SHA1. The private key material itself is never returned once set.

The filters are applied using an **AND** boolean operator, so depending on the combination of filters, they may become mutually exclusive. `id` must not be specified in combination with any of the others.

If more or less than a single match is returned by the search, the read fails. Ensure that your search is specific enough to return a single result.

## Example Usage

```terraform
data "fastly_tls_private_key" "example" {
  name = "example-key"
}
```

## Schema

### Optional

- `created_at` (String) Timestamp (GMT) when the private key was created.
- `id` (String) Fastly private key ID. Conflicts with all other filters.
- `key_length` (Number) The key length used to generate the private key.
- `key_type` (String) The algorithm used to generate the private key. Currently, the only allowed value is `RSA`.
- `name` (String) The name of the private key.
- `public_key_sha1` (String) The SHA1 digest of the private key's public key. Useful for safely identifying the key.

### Read-Only

- `replace` (Boolean) A recommendation from Fastly to replace this private key and all associated certificates.
