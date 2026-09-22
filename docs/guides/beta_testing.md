---
page_title: Beta Testing Guide
subcategory: "Guides"
---

## Beta Testing Guide

This provider is a ground-up rewrite of the Fastly Terraform provider
on HashiCorp's Plugin Framework, built around two parallel resource
families. This release contains one of them.

The **Automatic** family keeps the default version lifecycle behavior
you have today: the provider clones, validates, and activates a
service version for you during `terraform apply`. If that's how you
work with Fastly now, the beta is ready for you to test.

The **Explicit** family hands the version lifecycle back to you. If
you currently use `activate = false` or staging to control when
changes go live, wait until this family is ready for testing in mid-Q4
2026. It uses its own set of resources, some of which are registered
here and appear in the navigation, but they aren't ready to use yet.
Visit the [repository
README](https://github.com/fastly/terraform-provider-fastly-beta#resource-families)
for a full list.

Everything else in the provider is versionless: it works with either
family, and it's ready to test now.

This guide takes you through setting up the beta provider, running
`validate` against a copy of your existing configuration, using what
it reports to guide the translation until it validates, then
optionally exercising `plan` and `apply`. For the HCL changes
themselves, see the [HCL Syntax Changes](hcl_syntax_changes.md) guide.

### You can test without changing your Fastly service

**This is not a migration.** You're not moving production state, but
instead building a parallel configuration in a separate directory with
its own state file.

If you're short on time, you can stop after `terraform validate`. That
tells you what changes your configuration would need, and helps
surface gaps or issues we need to address. Each step after that tells
you more about how the provider behaves in practice.

### Before you start

You need three things:

- **Terraform.** We recommend a recent release, but any 1.x release
  works with the Automatic family.
- **A Fastly API token**, for the `plan` step onward.
- **Your existing HCL configuration for the Fastly provider**, to work
  from. Real configurations are best for surfacing gaps.

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
`api_token` argument.

~> **Important:** Several provider settings changed from the legacy provider, so a `provider` block copied across as-is will not work. Refer to the table below to update your attributes and environment variables as needed.

|Legacy provider|This provider|
|---|---|
|`source = "fastly/fastly"`|`source = "fastly/fastly-beta"`|
|`api_key`|`api_token`|
|`FASTLY_API_KEY`|`FASTLY_API_TOKEN`|
|`base_url`, `no_auth`, `force_http2`|Not available|

Then:

```bash
terraform init
```

The provider is still changing during the beta, so we suggest leaving
the version unpinned and running `terraform init -upgrade` to pick up
new releases.

### Translate and validate your configuration

Copy your configuration into the new directory. Before anything else,
rename the service resources:

|Legacy provider|This provider|
|---|---|
|`fastly_service_vcl`|`fastly_service_cdn_auto`|
|`fastly_service_compute`|`fastly_service_compute_auto`|

To find them:

```bash
grep -rnE 'resource "fastly_service_(vcl|compute)"' . --include=*.tf
```

Rename these first, because `fastly_service_compute` still exists in
this provider, as the **Explicit** family counterpart to
`fastly_service_compute_auto`. It accepts none of the nested blocks a
legacy Compute service uses, so a block left unrenamed reports errors
on `package`, `domain`, and `backend` rather than on the resource type
— taken at face value, those errors lead you to delete configuration
you need.

With the service resources renamed, let `terraform validate` find the
rest:

```bash
terraform validate
```

It checks your configuration against the provider's schema and reports
what doesn't match: renamed resources, arguments that no longer exist,
and attributes that moved into nested blocks. Work through it
alongside the [HCL Syntax Changes](hcl_syntax_changes.md) guide,
running it again as you go — Terraform doesn't look inside a block it
has already rejected, so the findings arrive in layers rather than all
at once.

Most of what it reports will be one of two things:

- **A renamed or moved argument.** Covered in the
  [HCL Syntax Changes](hcl_syntax_changes.md) guide.
- **Something unexpected.** That may be a bug. Reporting it will help
  us improve the provider.

### Review the plan

When `validate` is clean, set your token and run a plan:

```bash
export FASTLY_API_TOKEN=...
terraform plan
```

Read the plan output as a review of your translation: the resource
count should roughly match what you expect, and the attribute values
should look like your existing configuration.

### Exercise the workflow

Optional. We suggest using a **non-production service** — either a
service you already run outside production, or a copy of a production
service created for this test.

The steps below assume the second case: the configuration creates a
new service, and no existing state is involved.

Pointing this provider at a service Terraform already manages is a
different exercise: a real migration rather than a parallel test.
State can't be carried across from the legacy provider, and we don't
have documentation or tooling for that path yet, so it's outside what
this guide covers.

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

[Open an
issue](https://github.com/fastly/terraform-provider-fastly-beta/issues)
in the provider repository, or reach out to your Fastly account team.

For bugs, the most useful report includes the relevant configuration
snippet, what you expected, what happened, and your Terraform and
provider versions.
