---
page_title: "fastly_tsig_key Resource - fastly"
subcategory: ""
description: |-
  Provides a Fastly TSIG Key.
---

# fastly_tsig_key (Resource)

Provides a Fastly TSIG Key.

This resource is versionless: it is not tied to a service version and is managed independently of any `fastly_service_cdn` or `fastly_service_compute` resource.

## Example Usage

```terraform
resource "fastly_tsig_key" "example" {
  name      = "example.com."
  algorithm = "hmac-sha256"
  secret = {
    value = "c2VjcmV0a2V5MTIzNDU2Nzg="
  }
  description = "My TSIG key"
}
```

Deleting a TSIG key that's still referenced by a `fastly_dns_zone`'s `inbound_tsig_key_id` requires removing that reference first, in a separate `terraform apply`.

## Schema

### Required

- `algorithm` (String) The algorithm of the TSIG key. One of: `hmac-sha224`, `hmac-sha256`, `hmac-sha384`, `hmac-sha512`.
- `name` (String) The name of the TSIG key.
- `secret` (Attributes) The TSIG key's shared secret. (see [below for nested schema](#nestedatt--secret))

### Optional

- `description` (String) A freeform descriptive note.

### Read-Only

- `id` (String) TSIG Key Identifier (UUID).

<a id="nestedatt--secret"></a>
### Nested Schema for `secret`

Required:

- `value` (String, Sensitive) The Base64 encoded secret key. Sensitive key material is not returned once set, so it cannot be read back after creation and will not be populated after a `terraform import`.

## Import

Fastly TSIG Keys can be imported using their Key ID, e.g.

```shell
terraform import fastly_tsig_key.example xxxxxxxxxxxxxxxxxxxx
```
