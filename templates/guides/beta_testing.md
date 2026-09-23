---
page_title: Beta Testing Guide
subcategory: "Guides"
---

## Beta Testing Guide

This provider is a ground-up rewrite of the Fastly Terraform provider on
HashiCorp's Plugin Framework, built around two parallel resource families.
This release contains one of them.

The **Automatic** family retains the legacy provider's [default activation
behavior](https://registry.terraform.io/providers/fastly/fastly/latest/docs/resources/service_vcl#activation-and-staging):
it clones, validates, and activates a service version for you during
`terraform apply`. If that's how you work with Fastly now, the beta is ready
for you to test.

If you use `activate = false` or staging to control when changes go live, wait
for the **Explicit** family, ready for testing in mid-Q4 2026. Its resources
appear in the navigation here but aren't usable yet; the [repository
README](https://github.com/fastly/terraform-provider-fastly-beta#resource-families)
lists them.

This guide takes one existing service and puts it under the beta provider's
management, using Terraform's [configuration
generation](https://developer.hashicorp.com/terraform/language/import/generating-configuration)
to build the configuration from the live service rather than by hand. Nothing
you run before [step 7](#7-apply) changes anything on Fastly, so you can stop
at any point up to there.

### Before you start

You need:

- **Terraform 1.5 or later.** Configuration generation is not available in
  earlier releases.
- **A non-production Fastly service** that's representative of how you use
  Fastly.
- **A Fastly API token.**
- **Your existing Fastly configuration**, to compare against. You won't edit
  it until the last step.

Work in a **new, empty directory with its own state file**. Do not point this
provider at your existing Terraform state.

### 1. Create a working directory

In the new directory, create a file containing only the provider requirement
and configuration:

```hcl
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly-beta"
      version = "0.1.3"
    }
  }
}

provider "fastly" {}
```

Replace `0.1.3` with the current version from the [registry
page](https://registry.terraform.io/providers/fastly/fastly-beta/latest). Pin
it, and keep the `.terraform.lock.hcl` that `terraform init` writes. The beta
changes often, and a pinned version is what lets anyone reproduce a problem
you hit. When you want to retest against a newer beta, run
`terraform init -upgrade` deliberately.

~> **Important:** Several provider settings changed from the legacy provider, so a `provider` block copied across as-is will not work. The table below lists the differences.

|Legacy provider|This provider|
|---|---|
|`source = "fastly/fastly"`|`source = "fastly/fastly-beta"`|
|`api_key`|`api_token`|
|`FASTLY_API_KEY`|`FASTLY_API_TOKEN`|
|`base_url`, `no_auth`, `force_http2`|Not available|

The provider reads `FASTLY_API_TOKEN` from the environment, or takes an
`api_token` argument.

```bash
export FASTLY_API_TOKEN=<your-token>
terraform init
```

### 2. Add an import block

Add a second file containing an `import` block for the service, and one for
each resource attached to it that lives outside the service version, such as
domains, ACLs, and config stores. Do not add any `resource` blocks —
Terraform generates those for you in [step 3](#3-generate-a-configuration).

```hcl
import {
  provider = fastly
  to       = fastly_service_cdn_auto.example
  id       = "<service-id>"
}

import {
  provider = fastly
  to       = fastly_domain.example
  id       = "<domain-id>"
}
```

The service resource types are named differently here:

|Legacy provider|This provider|
|---|---|
|`fastly_service_vcl`|`fastly_service_cdn_auto`|
|`fastly_service_compute`|`fastly_service_compute_auto`|

~> **Important:** Use the `_auto` names. `fastly_service_compute` also exists in this provider, as the Explicit family counterpart, and it accepts none of the nested blocks a legacy Compute service uses.

The `provider = fastly` line is required here and easy to miss. A resource
type that appears only in an `import` block doesn't resolve through
`required_providers`, so without it Terraform looks for a provider called
`hashicorp/fastly` and fails with an error naming a provider you never
configured. You'll remove the line again in
[step 5](#5-finish-the-configuration).

Your existing configuration already knows the IDs:

```bash
# in your existing configuration's directory
terraform state list
terraform state show fastly_service_vcl.example
```

`state list` gives you the resource addresses — use yours in place of
`fastly_service_vcl.example` here, and wherever `example` appears in this
guide. In the output of `state show`, `id` is the service ID. Domain IDs come
from the same place.

### 3. Generate a configuration

```bash
terraform plan -generate-config-out=generated.tf
```

This reads the live service and writes a configuration for it. It does not
create a state file and does not change the service. Terraform reports the
generated configuration as experimental; that warning is expected.

### 4. Review what Terraform generated

`generated.tf` is a starting template, not a finished configuration. Read it
against your existing HCL and the [HCL Syntax
Changes](hcl_syntax_changes.md) guide.

Terraform writes what the provider read back from the service, which has two
consequences worth knowing before you read the file:

- **Nested blocks arrive verbose.** A `backend` block comes back with every
  attribute spelled out at its default — `auto_loadbalance`, `max_conn`,
  `weight`, and the rest. Deleting the ones you don't set makes the
  configuration easier to read and changes nothing. The same applies to any
  other nested block on your service, including `settings`.
- **Anything that exists only in Terraform is missing.** `force_destroy` and
  `reuse` have no counterpart on the service, so they come back empty and
  aren't written. Neither is a Compute `package`, which lives on your disk
  rather than on the service. [Step 5](#5-finish-the-configuration) covers
  what to put back.

Compare the generated configuration against your existing one, even if you
plan to delete this directory afterward. The generated file is the beta
provider's own reading of your service. Where the two disagree, the provider
has either a bug or an undocumented change, and both are worth
[reporting](#send-us-feedback).

### 5. Finish the configuration

Two edits are always needed:

1. **Remove the `provider` argument** from every `import` block. Terraform
   accepts it only while generating; now that `generated.tf` exists, the same
   line is an error.
2. **Add the `package` block, for a Compute service.** Point it at your built
   package:

    ```hcl
    package {
      filename = "./package.tar.gz"
    }
    ```

~> **Important:** `terraform validate` passes on a Compute configuration with no `package` block, so it will not catch a missing package for you. Check for it yourself.

Add `force_destroy = true` to the service resource if you want to be able to
delete the service when you're finished. Terraform cannot destroy a service
that has an active version without it.

The rest only matters if you intend to keep the service on the beta provider.
A test directory you're going to delete works without them:

- **Replace literal IDs with references.** Generated resources refer to each
  other by ID string — a `fastly_domain` arrives with
  `service_id = "<service-id>"` rather than a reference to the service
  resource.
- **Restore variables, locals, and module structure** from your existing
  configuration. Generation writes literal values throughout, so a
  configuration you built from modules comes back flat.

### 6. Validate and plan

```bash
terraform fmt
terraform validate
terraform plan
```

Read the plan as a review of the generated configuration. Terraform reports an
update in place on the service resource: `force_destroy` and `reuse` being
set, and the version attributes recomputed. Those are Terraform's own
bookkeeping — they are recorded in state and send nothing to Fastly. Anything
else in that update is a real difference between your configuration and the
live service.

Do not continue until the plan contains only the imports you intended and no
unexpected changes to the service.

### 7. Apply

This is the step that takes over managing the service, and the first one that
changes anything. Use a **non-production** service. If it's a Compute service,
check that the package on disk is the one you want running — this apply
uploads it and activates it, replacing what the service is serving now.

```bash
terraform apply
```

A CDN service should stay on the version it was already running. A Compute
service gets a new one: taking over the service re-uploads the package, even
when it matches what's already deployed.

Then confirm the configuration converges:

```bash
terraform plan
```

It should report no changes. If it reports changes instead, your configuration
and the service disagree about something, and every apply from here on will
try to reconcile it — so it's worth [telling us](#send-us-feedback) what the
plan says.

~> **Important:** While you're testing, don't run your legacy configuration against this service. Both configurations now describe it, and applying from the legacy one would fight the beta provider.

### 8. Exercise the workflow

Now make some changes. This tests the parts a plan cannot: the clone,
validate, and activate cycle that runs on every change.

Terraform's output shows that the version attributes changed, but not their
new values — `terraform state show` reports `active_version`, and the Fastly
UI shows what actually landed in that version. A few things worth trying:

1. **Change one attribute** on one backend, and read the plan before you
   apply. The `backend` block should show that one attribute changing, with
   the rest reported as unchanged — not the whole block being removed and
   re-added, which is how the legacy provider renders it. Applying should then
   give you one new version, activated, containing only that change.
2. **Remove a nested block** and apply. The removal should land in a single
   new version.
3. **Run `terraform plan` again** with nothing changed. It should report no
   changes.
4. **If you manage several services**, change one and confirm the others are
   untouched.

The [`orchestration-cdn-auto`
example](https://github.com/fastly/terraform-provider-fastly-beta/tree/main/examples/orchestration-cdn-auto)
in the provider repository has a longer list of suggested tests with expected
outcomes.

### 9. When you're done

**If you stopped before applying**, nothing on Fastly changed. Delete the
directory.

**If you applied**, take stock of three things before you decide what to do
with the service:

- The beta provider's state now manages the service.
- The service may be running a version the beta provider activated. Taking
  over a Compute service always produces one. A CDN service only gets one if
  you made changes in [step 8](#8-exercise-the-workflow).
- Your legacy state no longer reflects the service.

**To hand the service back to the legacy provider**, work in your existing
configuration's directory:

```bash
# in your existing configuration's directory
terraform plan
terraform apply
```

The plan proposes bringing the service back in line with your legacy
configuration. Read it before you apply rather than assuming it: for a Compute
service it re-uploads the package your legacy configuration points at, and for
either type it undoes anything you changed in
[step 8](#8-exercise-the-workflow).

Then delete the beta directory.

~> **Important:** Delete the beta directory. Do not run `terraform destroy` in it — the beta provider manages the real service, so a destroy deletes the service itself, not just your test setup.

**To keep the service on the beta provider**, remove **every** object you
imported from the legacy state, not just the service, and delete their
resource blocks from that configuration:

```bash
# in your existing configuration's directory
terraform state rm -dry-run fastly_service_vcl.example
terraform state rm fastly_service_vcl.example
terraform state rm fastly_domain.example
```

`state rm` removes only the resources you name, and changes nothing in
Fastly. Delete the resource blocks too, along with anything that references
them — otherwise the next legacy plan will try to create the service again.

### Send us feedback

The provider isn't finished, which is the useful part: a problem you report
now can still be fixed by changing the design. Once the provider is released,
the same fix means a breaking change for everyone already using it, and the
bar for making it is much higher.

Worth reporting:

- **Something didn't translate** — a resource, argument, or block you need
  that isn't here, or isn't documented.
- **Something translated but misbehaved** — wrong plan output, a failed
  apply, a plan that won't converge, unexpected version behavior.
- **Something was confusing** — unclear errors, gaps in this guide or the
  syntax guide.

[Open an
issue](https://github.com/fastly/terraform-provider-fastly-beta/issues) in the
provider repository, or reach out to your Fastly account team if you'd rather
not report it publicly.

For bugs, the most useful report includes the relevant configuration snippet,
what you expected, what happened, and your Terraform and provider versions.
