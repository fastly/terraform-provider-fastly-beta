---
page_title: "fastly_service_dynamic_snippet_content Resource - fastly"
subcategory: ""
description: |-
  Fastly dynamic VCL snippet content resource. Updates versionless dynamic snippet code directly by service ID and snippet ID.
---

# fastly_service_dynamic_snippet_content (Resource)

Fastly dynamic VCL snippet content resource. Updates versionless dynamic snippet
code directly by service ID and snippet ID.

Dynamic VCL snippets have two lifecycle parts:

- metadata such as `name`, `type`, and `priority`, which is versioned service
  configuration
- content, which is versionless

In the automatic compatibility family, dynamic snippet metadata is managed by
the `dynamic_snippet` block on `fastly_service_cdn_auto`.

In the explicit/default first-class resource family, dynamic snippet metadata is
managed by `fastly_service_dynamic_vcl_snippet`.

Updating this resource's `content` does not clone, validate, stage, or activate a
service version. Changes are applied immediately to the dynamic snippet.

Both metadata resources also accept their own optional `content` attribute, as
an alternative to managing content here. If other VCL - for example a main
VCL's `include "snippet::name"` - references code that must exist in the
snippet, set it there instead of here: a metadata resource's `content` seeds
the snippet when it's first created, so the very first service version - the
one validated and activated during that same create - actually contains it.
This resource can't help with that specific case, since it depends on the
metadata resource's computed `snippet_id`, which isn't available until after
that first version has already been validated and activated.

If a metadata resource's `content` is set, it keeps enforcing that same value
on every subsequent apply too, not just at creation - so don't configure
content on both a metadata resource and this resource for the same snippet;
they'll fight over it on every apply that touches either one.

## Example Usage with automatic compatibility metadata

```terraform
resource "fastly_service_cdn_auto" "example" {
  name = "example"

  domain {
    name = "www.example.com"
  }

  backend {
    name    = "origin"
    address = "origin.example.com"
    port    = 443
    use_ssl = true
  }

  dynamic_snippet {
    name     = "block_scrapers"
    type     = "recv"
    priority = 100
  }

  force_destroy = true
}

resource "fastly_service_dynamic_snippet_content" "block_scrapers" {
  service_id = fastly_service_cdn_auto.example.id
  snippet_id = {
    for s in fastly_service_cdn_auto.example.dynamic_snippet : s.name => s.snippet_id
  }["block_scrapers"]

  content = file("${path.module}/block_scrapers.vcl")
}
```

## Example Usage with explicit/default metadata

In the explicit/default workflow, dynamic snippet metadata is managed by
`fastly_service_dynamic_vcl_snippet`. The content resource depends on the
computed `snippet_id`, so Terraform creates the metadata container first and then
applies versionless content.

```terraform
resource "fastly_service_cdn" "example" {
  name = "example"

  force_destroy = true
}

resource "fastly_service_dynamic_vcl_snippet" "block_scrapers" {
  service_id = fastly_service_cdn.example.id
  version    = 1

  name     = "block_scrapers"
  type     = "recv"
  priority = 100
}

resource "fastly_service_dynamic_snippet_content" "block_scrapers" {
  service_id = fastly_service_cdn.example.id
  snippet_id = fastly_service_dynamic_vcl_snippet.block_scrapers.snippet_id

  content = file("${path.module}/block_scrapers.vcl")
}
```

## Schema

### Required

- `content` (String) The dynamic VCL snippet code. Updates are versionless and take effect immediately without cloning or activating a service version.
- `service_id` (String) Fastly service ID.
- `snippet_id` (String) The Fastly-generated ID of the dynamic VCL snippet whose content is managed by this resource.

### Optional

- `manage_snippets` (Boolean) Whether Terraform should re-apply content drift and clear snippet content on destroy. Default `false`.

### Read-Only

- `id` (String) Terraform resource identifier.

## Import

Import requires the service ID and dynamic snippet ID:

```shell
terraform import fastly_service_dynamic_snippet_content.block_scrapers SERVICE_ID/SNIPPET_ID
```

Example:

```shell
terraform import fastly_service_dynamic_snippet_content.block_scrapers SU1Z0isxPaozGVKXdv0eY/abc123
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Dynamic
snippet content is versionless and updates are applied directly to the dynamic
snippet.
