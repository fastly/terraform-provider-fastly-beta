---
page_title: "fastly_service_dynamic_vcl_snippet Resource - fastly"
subcategory: ""
description: |-
  Fastly dynamic VCL snippet metadata resource. Writes the versioned dynamic snippet container directly to the specified writable CDN service version.
---

# fastly_service_dynamic_vcl_snippet (Resource)

Fastly dynamic VCL snippet metadata resource. Writes the versioned dynamic
snippet container directly to the specified writable CDN service version.

This resource is part of the explicit/default first-class resource family. It
manages the versioned metadata for a dynamic VCL snippet:

- `name`
- `type`
- `priority`
- computed `snippet_id`

Dynamic snippet content itself is versionless, and is normally managed
separately, on an ongoing basis, with `fastly_service_dynamic_snippet_content`.
This resource's own optional `content` attribute is an alternative: it seeds
the snippet's content when this resource is first created, which is needed if
other VCL (for example an `include "snippet::name"`) references code that
must exist in the snippet for the service version to compile.

If set, this attribute is re-pushed - silently overwriting whatever
`fastly_service_dynamic_snippet_content` currently holds for the same
snippet - whenever this resource is updated for *any* reason, not only when
`content` itself changes: for example a `priority` or `type` change, or
retargeting `version` to a different writable service version. See
[`fastly_service_dynamic_snippet_content`'s documentation](https://registry.terraform.io/providers/fastly/fastly-beta/latest/docs/resources/service_dynamic_snippet_content#configuring-content-on-both-a-metadata-resource-and-this-resource)
for exactly how the two resources interact if both are configured for the
same snippet - there is no safe way to have both manage the same snippet's
content concurrently.

Use this resource when you want to manage dynamic VCL snippet metadata explicitly
against a known writable service version. For automatic service version cloning,
validation, and activation, use the nested `dynamic_snippet` block on
`fastly_service_cdn_auto`.

## Example Usage

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

- `name` (String) A name that is unique across regular and dynamic VCL snippet configuration blocks. Changing this attribute will delete and recreate the snippet.
- `service_id` (String) Fastly service ID.
- `type` (String) The location in generated VCL where the dynamic snippet should be placed. Must be one of `init`, `recv`, `hash`, `hit`, `miss`, `pass`, `fetch`, `error`, `deliver`, `log`, or `none`.
- `version` (Number) Writable Fastly service version to modify.

### Optional

- `content` (String) The VCL code the dynamic snippet is seeded with when it is first created, so the very first service version - the one validated and activated during that create - actually contains it. Set this when other VCL (for example an `include "snippet::name"`) references code that must exist in the snippet for the version to compile. If set, this attribute silently overwrites whatever `fastly_service_dynamic_snippet_content` currently holds for the same snippet, any time this resource is next updated for any reason - not only when this attribute's own value changes; see `fastly_service_dynamic_snippet_content`'s documentation for exactly when that happens and how to avoid it. Leave unset to manage all content, including the initial value, via `fastly_service_dynamic_snippet_content` instead.
- `priority` (Number) Priority determines execution order. Lower numbers execute first. Default `100`.

### Read-Only

- `id` (String) Terraform resource identifier.
- `snippet_id` (String) The Fastly-generated dynamic snippet ID. Use this value with `fastly_service_dynamic_snippet_content` to manage versionless snippet code.

## Import

Import requires the service ID, service version, and dynamic VCL snippet name:

```shell
terraform import fastly_service_dynamic_vcl_snippet.block_scrapers SERVICE_ID/VERSION/SNIPPET_NAME
```

Example:

```shell
terraform import fastly_service_dynamic_vcl_snippet.block_scrapers SU1Z0isxPaozGVKXdv0eY/3/block_scrapers
```

## Version lifecycle

This resource does not clone, activate, or stage service versions. Use explicit
service-version lifecycle actions to clone, validate, stage, or activate a
service version.

Only the dynamic snippet metadata is versioned. Dynamic snippet content itself
is versionless and, other than the optional creation-time seed described
above, is managed separately with `fastly_service_dynamic_snippet_content`.

Terraform can apply metadata and content in one run when the content resource
references the computed `snippet_id`. The metadata resource still writes to the
configured writable service version, while content updates are applied directly
by snippet_id.
