---
page_title: Beta Testing Guide
subcategory: "Guides"
---

## Beta Testing Guide

This provider is a ground-up rewrite of the Fastly Terraform provider
on HashiCorp's Plugin Framework, built around two parallel resource
families. This release contains one of them.

The **Automatic** family carries over the [default activation
behavior](https://registry.terraform.io/providers/fastly/fastly/latest/docs/resources/service_vcl#activation-and-staging)
from the legacy provider: it clones, validates, and activates a service
version for you during `terraform apply`. If that's how you work with
Fastly now, the beta is ready for you to test.

If you use `activate = false` or staging to control when changes go
live, wait for the **Explicit** family, ready for testing in mid-Q4
2026. Its resources appear in the navigation here but aren't usable
yet; the [repository
README](https://github.com/fastly/terraform-provider-fastly-beta#resource-families)
lists them.

All other resources in the provider work independently of the
activation workflow, and are ready to test now.

### Choose how far you want to go

Everything up to `terraform validate` runs entirely on your own
machine. You work from a copy of your configuration in a separate
directory with its own state file, and nothing contacts Fastly. If
you're short on time, you can stop there: it tells you what changes
your configuration would need, and surfaces anything that would block
your migration in time for us to fix it.

The next step is the first that contacts Fastly. You add an `import`
block for one of your services and run `terraform plan`, which shows
you the whole migration without writing anything — no state file, and
no change to the service. Any service is safe to read this way; the
work is in gathering the IDs, so pick something small to start.

Applying is where the provider takes over managing the service. Use a
**non-production** service, and from there you can make changes and watch
how it handles them.

### Before you start

You need:

- **Terraform.** Any 1.x release works through `terraform validate`.
  The steps after that need **Terraform 1.5 or later**.
- **Your existing HCL configuration for the Fastly provider**, to work
  from. Real configurations are best for surfacing gaps.
- **A Fastly API token**, from the `import` step onward.
- **A non-production service**, from the `apply` step onward. That's
  where the provider takes over managing it.

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
at once. Repeat until it reports success.

Most of what it reports will be one of two things:

- **A renamed or moved argument.** Covered in the
  [HCL Syntax Changes](hcl_syntax_changes.md) guide.
- **Something unexpected.** That may be a bug.
  [Reporting it](#send-us-feedback) will help us improve the provider.

### Import an existing service

From here on you need an API token. Nothing in this section changes
anything: the plan reads your service and writes no state.

Applying your translated configuration as it stands would try to
create a *second* service with the same domains, which Fastly won't
allow. So rather than creating anything, take over the service you
already have.

If the configuration you translated covers several services, narrow it
first. Terraform tries to *create* anything in the configuration you
don't import, which runs straight back into that conflict. Copy just
the blocks for the service you're importing — the service resource
plus anything that references it, usually through `service_id` — into
a directory of its own, and work from there.

Your existing configuration already knows the service ID:

```bash
# in your existing configuration's directory
terraform state list
```

```bash
# in your existing configuration's directory
terraform state show fastly_service_vcl.example
```

`state list` gives you the resource addresses — use yours in place of
`fastly_service_vcl.example` here, and wherever `example` appears
below. In the output of `state show`, `id` is the service ID. Domain
IDs come from the same place, if your configuration uses
`fastly_domain`.

Back in the new directory, add an `import` block for the service, and
one for each versionless resource attached to it — domains, ACLs,
config stores.

```hcl
import {
  to = fastly_service_cdn_auto.example
  id = "<service-id>"
}

import {
  to = fastly_domain.example
  id = "<domain-id>"
}
```

Then set your token and plan:

```bash
export FASTLY_API_TOKEN=<your-token>
terraform plan
```

This plan writes nothing — not even a state file. It reads the live
service and shows you what importing it would produce, so you can read
the whole migration before committing to any of it.

Read it as a review of your translation. Alongside the imports you
should see one small in-place update to the service: `force_destroy`
and `reuse` being set, and the version attributes recomputed. Those are
the provider's own bookkeeping and don't touch the service. Anything
else is a real difference between your configuration and the live
service. Those are worth reporting.

### Exercise the workflow

This is the step that takes over managing the service, so use a
**non-production** one. If it's a Compute service, check that the
package on disk is the one you want running — this apply uploads it
and activates it, replacing what the service is serving now.

When the plan looks right:

```bash
terraform apply
```

A CDN service should stay on the version it was already running. A
Compute service gets a new one: taking over the service re-uploads the
package, even when it matches what's already deployed. Either way, run
`terraform plan` once more and it should report no changes.

~> **Important:** While you're testing, don't run your legacy configuration against this service. Both configurations now describe it, and applying from the legacy one would fight the beta provider.

Now make some changes. This tests the parts a plan cannot: the clone,
validate, and activate cycle that runs on every change.

Terraform's output shows that the version attributes changed, but not
their new values — `terraform state show` reports `active_version`,
and the Fastly UI shows what actually landed in that version. A few
things worth trying:

1. **Change one attribute** on one backend, and read the plan before
   you apply. The `backend` block should show that one attribute
   changing, with the rest reported as unchanged — not the whole block
   being removed and re-added, which is how the legacy provider
   renders it. Applying should then give you one new version,
   activated, containing only that change.
2. **Remove a nested block** and apply. The removal should land in a
   single new version.
3. **Run `terraform plan` again** with nothing changed. It should report
   no changes. If it doesn't, that's a convergence bug and we want to
   hear about it.
4. **If you manage several services**, change one and confirm the others
   are untouched.

The [`orchestration-cdn-auto`
example](https://github.com/fastly/terraform-provider-fastly-beta/tree/main/examples/orchestration-cdn-auto)
in the provider repository has a longer list of suggested tests with
expected outcomes.

### When you're done

Your legacy configuration still has this service in its state, so
none of this is hard to undo: delete the beta directory and carry on
as you were.

If you decide to keep the service on the beta provider, remove it from
the legacy state and delete its resource block from that
configuration:

```bash
# in your existing configuration's directory
terraform state rm -dry-run fastly_service_vcl.example
terraform state rm fastly_service_vcl.example
```

`state rm` removes only the resource you name, and changes nothing in
Fastly. Delete the resource block too, along with anything that
references it — otherwise the next legacy plan will try to create the
service again.

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
