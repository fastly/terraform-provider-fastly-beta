---
page_title: Beta Testing Guide
subcategory: "Guides"
---

## Beta Testing Guide

This provider is a ground-up rewrite of the Fastly Terraform provider
on HashiCorp's Plugin Framework. It's designed to provide [two
parallel resource
families](https://github.com/fastly/terraform-provider-fastly-beta#resource-families).
This release contains the **Automatic** resource family, which keeps
the version lifecycle behavior you have today: the provider clones,
validates, and activates a service version for you during
`terraform apply`.

This guide walks through testing the beta against a configuration you
already run. It covers setup, checking your translated configuration,
and exercising the workflow. For the HCL changes themselves, see the
[HCL Syntax Changes](hcl_syntax_changes.md) guide.

**This is not a migration.** You are not moving production state. You
are building a parallel configuration in a separate directory, so you
can find out what a real migration would involve before committing to
one.

### You can test without creating a Fastly service

The most useful result — *does everything I do today translate?* —
costs nothing and creates nothing.

|Command|Needs an API token|Calls the Fastly API|Creates anything|
|---|---|---|---|
|`terraform init`|No|No|No|
|`terraform validate`|No|No|No|
|`terraform plan`|Yes|Reads only|No|
|`terraform apply`|Yes|Yes|Yes|

`terraform validate` checks your configuration against the provider's
schema — resource names, arguments, types, block structure. It will
find every renamed resource and every moved attribute.

That is where most of the feedback we need comes from. If you can only
do this part, please still do it and tell us what you find.

If you also have a non-production service, applying against it tests
the parts a plan can't reach. That's covered further down, and it's
optional.

### Who should test now

Good fit if you use the legacy provider with automatic activation —
`activate = true`, or the default.

**Worth waiting** if you use `activate = false` or staging to control
when changes go live. The Automatic family always activates. The
**Explicit** resource family is designed for controlled activation and
is targeted for beta testing in mid-Q4 2026. It uses its own set of
resources, so a configuration translated for the Automatic family now
would not carry over — it's worth waiting for that release rather than
translating twice. If you'd like to help shape the Explicit design
while it's in development, reply on the [community
forum](https://community.fastly.com/t/introducing-the-public-beta-of-the-new-fastly-terraform-provider/4494)
or talk to your account team about becoming a design partner.

### Before you start

You need three things:

- **Terraform.** Any 1.x release works with the Automatic family. We've
  confirmed the provider loads and configurations validate on 1.0.11,
  1.5.7, 1.13.0, and 1.16.1, and that a full apply and destroy cycle
  succeeds on both 1.5.7 and 1.16.1 — so you shouldn't need to upgrade
  Terraform to take part.
- **A Fastly API token**, for the `plan` step onward. Not needed to
  begin.
- **A copy of a configuration you already run.** A real one. Trimmed
  examples won't surface the gaps we're looking for.

Work in a **new directory with its own state file**. Do not point this
provider at your existing Terraform state.

### Set up the provider

```hcl
terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly-beta"
    }
  }
}

provider "fastly" {}
```

The provider reads `FASTLY_API_TOKEN` from the environment, or takes an
`api_token` argument. Note the changes from the legacy provider:

|Legacy provider|This provider|
|---|---|
|`source = "fastly/fastly"`|`source = "fastly/fastly-beta"`|
|`api_key`|`api_token`|
|`FASTLY_API_KEY`|`FASTLY_API_TOKEN`|
|`base_url`, `no_auth`, `force_http2`|Not available|

The provider is still changing during the beta, so we suggest leaving
the version unpinned and running `terraform init -upgrade` to pick up
new releases.

Then:

```bash
terraform init
```

### Translate your configuration

Copy your configuration into the new directory and work through the
[HCL Syntax Changes](hcl_syntax_changes.md) guide. Two things are worth
doing before anything else.

**Rename the service resources.** `fastly_service_vcl` becomes
`fastly_service_cdn_auto`, and `fastly_service_compute` becomes
`fastly_service_compute_auto`.

Do `fastly_service_compute` first. That name still exists in this
provider, naming a service resource in the **Explicit** family, which
is still in development and not ready for testing. It requires only
`name`, so a block copied across without renaming can validate and
apply — quietly creating the wrong kind of object.
`fastly_service_vcl` is safe by comparison: it no longer exists, so
Terraform rejects it outright. Find both:

```bash
grep -rnE 'resource "fastly_service_(vcl|compute)"' . --include=*.tf
```

Other Explicit-family resources are registered in this provider but
aren't ready to use either — the [README lists
them](https://github.com/fastly/terraform-provider-fastly-beta#resource-families).
Everything else works with the Automatic family.

**Remove `activate` and `stage`.** The Automatic family has no
equivalent; it always activates.

You don't need to get the translation perfect before moving on. The
next step will tell you what's left.

### Check your translation

```bash
terraform validate
```

No token needed. Work through what it reports — most findings will be
one of two things:

- **A renamed or moved argument.** Covered in the
  [HCL Syntax Changes](hcl_syntax_changes.md) guide.
- **Something unexpected.** That may be a bug, and it's the most
  valuable thing you can report.

When `validate` is clean, set your token and run a plan:

```bash
export FASTLY_API_TOKEN=...
terraform plan
```

Read the plan output as a review of your translation: the resource
count should roughly match what you expect, and the attribute values
should look like your existing configuration.

### Exercise the workflow

Optional, and recommended against a **non-production service**.

Applying tests the parts a plan cannot: the clone, validate, and
activate cycle that runs on every change. A few things worth trying,
checking the service version in the Fastly UI or API as you go:

1. **Apply once.** The service should be created and its first version
   activated.
2. **Change one attribute** on one backend, and read the plan before
   you apply. The `backend` block should show that one attribute
   changing, with the rest reported as unchanged — not the whole block
   being removed and re-added, which is how the legacy provider
   renders it. Applying should then give you one new version,
   activated, containing only that change.
3. **Remove a nested block** and apply. The removal should land in a
   single new version.
4. **Run `terraform plan` again** with nothing changed. It should report
   no changes. If it doesn't, that's a convergence bug and we want to
   hear about it.
5. **If you manage several services**, change one and confirm the others
   are untouched.

The [`orchestration-cdn-auto`
example](https://github.com/fastly/terraform-provider-fastly-beta/tree/main/examples/orchestration-cdn-auto)
in the provider repository has a longer list of suggested tests with
expected outcomes.

### Send us feedback

This is the point of the beta, and every kind of report is useful:

- **Something didn't translate** — a resource, argument, or block you
  need that isn't here, or isn't documented.
- **Something translated but misbehaved** — wrong plan output,
  a failed apply, a plan that won't converge, unexpected version
  behavior.
- **Something was confusing** — unclear errors, gaps in this guide or
  the syntax guide.

Reach us by [opening an
issue](https://github.com/fastly/terraform-provider-fastly-beta/issues),
posting on the [Fastly community forum](https://community.fastly.com/c/terraform/26),
or through your Fastly account team.

For bugs, the most useful report includes the relevant configuration
snippet, what you expected, what happened, and your Terraform and
provider versions.
