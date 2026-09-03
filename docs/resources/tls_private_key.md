---
page_title: "fastly_tls_private_key Resource - fastly"
subcategory: ""
description: |-
  Provisions a private key for use with Fastly's TLS/SSL support. Private keys are versionless and independent of any service-version lifecycle. The resource is immutable: any change to name or private_key destroys and recreates it.
---

# fastly_tls_private_key (Resource)

Provisions a private key for use with Fastly's TLS/SSL support. Private keys are versionless and independent of any service-version lifecycle. The resource is immutable: any change to `name` or `private_key` destroys and recreates it.

Private key material is not returned once it has been uploaded, so `private_key.pem` cannot be verified after a `terraform import` and will not be present in state until the next apply that (re)creates the resource.

## Example Usage

```terraform
resource "fastly_tls_private_key" "example" {
  name = "example-key"

  private_key = {
    pem = file("example.com.key")
  }
}
```

## Schema

### Required

- `name` (String) A customizable name for your private key.
- `private_key` (Attributes) The private key material. Updating a private key in place is not supported, so any change here replaces the resource. (see [below for nested schema](#nestedatt--private_key))

### Read-Only

- `created_at` (String) Timestamp (GMT) when the private key was created.
- `id` (String) Alphanumeric string identifying a TLS private key.
- `key_length` (Number) The key length used to generate the private key.
- `key_type` (String) The algorithm used to generate the private key. Currently, the only allowed value is `RSA`.
- `public_key_sha1` (String) The SHA1 digest of the private key's public key. Useful for safely identifying the key.
- `replace` (Boolean) A recommendation from Fastly to replace this private key and all associated certificates.

<a id="nestedatt--private_key"></a>
### Nested Schema for `private_key`

Required:

- `pem` (String, Sensitive) Private key in PEM format. Sensitive key material is not returned once set, so it cannot be read back after creation and will not be populated after a `terraform import`.

## Import

A TLS private key can be imported using its ID, e.g.

```shell
terraform import fastly_tls_private_key.example xxxxxxxx
```

`private_key.pem` is not returned by the API and so is not populated by import; the next `terraform apply` will show a forced replacement unless `private_key.pem` in your configuration exactly matches the key data of the private key you are importing.
