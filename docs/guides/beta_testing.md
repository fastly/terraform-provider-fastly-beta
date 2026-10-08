---
page_title: Beta Testing Guide
subcategory: "Guides"
---

## Beta Testing Guide

This provider is a ground-up rewrite of the Fastly Terraform provider on
HashiCorp's Plugin Framework, built around two parallel resource families.

The **Automatic** family retains the legacy provider's [default activation
behavior](https://registry.terraform.io/providers/fastly/fastly/latest/docs/resources/service_vcl#activation-and-staging):
it clones, validates, and activates a service version for you during
`terraform apply`.

The **Explicit** family exposes service configuration as independent,
first-class Terraform resources. You select the service version each resource
targets, and the provider does not clone or activate versions during normal
resource CRUD. This family is intended for workflows where version lifecycle is
controlled separately, including workflows that previously used
`activate = false` or staging.

This guide shows how to adopt one existing non-production service with either
family:

- [Test the Automatic family](#test-the-automatic-family) when you want
  provider-managed cloning and activation.
- [Test the Explicit family](#test-the-explicit-family) when you want
  first-class resources and caller-managed service versions.

Use a **new, empty working directory with its own Terraform state** for either
path. Do not point the beta provider at your existing Terraform state.

### Before you start

You need:

- **A non-production Fastly service.**
- **A Fastly API token.**
- **Your existing Fastly configuration**, if the service is already managed by
  Terraform, so that you can compare it with generated configuration.
- **Terraform 1.5 or later** for the Automatic-family configuration-generation
  workflow.
- **Terraform 1.14 or later** for the Explicit-family `terraform query`
  workflow.

The provider reads `FASTLY_API_TOKEN` from the environment, or takes an
`api_token` argument.

~> **Important:** Several provider settings changed from the legacy provider, so a `provider` block copied across as-is will not work.

|Legacy provider|This provider|
|---|---|
|`source = "fastly/fastly"`|`source = "fastly/fastly-beta"`|
|`api_key`|`api_token`|
|`FASTLY_API_KEY`|`FASTLY_API_TOKEN`|
|`base_url`, `no_auth`, `force_http2`|Not available|

## Test the Automatic family

Use this path if you want the beta provider to manage service-version cloning,
validation, and activation during `terraform apply`.

### 1. Create an Automatic-family working directory

In a new directory, create a file containing only the provider requirement and
configuration:

```hcl
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly-beta"
      version = "<current-version>"
    }
  }
}

provider "fastly" {}
```

Replace `<current-version>` with the current version from the [Registry
page](https://registry.terraform.io/providers/fastly/fastly-beta/latest). Pin
it, and keep the `.terraform.lock.hcl` that `terraform init` writes. The beta
changes often, and a pinned version is what lets anyone reproduce a problem
you hit. When you want to retest against a newer beta, update the pinned
version first, then run `terraform init -upgrade` deliberately.

```bash
export FASTLY_API_TOKEN=<your-token>
terraform init
```

### 2. Add import blocks

Add a second file containing an `import` block for the service, and one for
each resource attached to it that lives outside the service version, such as
domains, ACLs, and config stores. Do not add any `resource` blocks — Terraform
generates those for you in [step 3](#3-generate-an-automatic-family-configuration).

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

~> **Important:** Use the `_auto` names. `fastly_service_compute` also exists in this provider as the Explicit-family counterpart, and it accepts none of the nested blocks a legacy Compute service uses.

The `provider = fastly` line is required here and easy to miss. A resource
type that appears only in an `import` block does not resolve through
`required_providers`, so without it Terraform looks for a provider called
`hashicorp/fastly` and fails with an error naming a provider you never
configured. You'll remove the line again in
[step 5](#5-finish-the-automatic-family-configuration).

You can find the IDs in your existing Terraform state or in Fastly's web
interface controls.

```bash
# in your existing configuration's directory
terraform state list
terraform state show fastly_service_vcl.example
```

`state list` gives you the resource addresses — use yours in place of
`fastly_service_vcl.example` here, and wherever `example` appears in this
guide. In the output of `state show`, `id` is the service ID. Domain IDs come
from the same place.

### 3. Generate an Automatic-family configuration

```bash
terraform plan -generate-config-out=generated.tf
```

This reads the live service and writes a configuration for it. It does not
create a state file and does not change the service. Terraform reports the
generated configuration as experimental; that warning is expected.

The output path must not already exist. If you want to generate the
configuration again, remove `generated.tf` first or choose a different output
path.

### 4. Review the Automatic-family configuration

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
- **Some Terraform-only or local inputs are missing.** `force_destroy` and
  `reuse` have no counterpart in the Fastly API, so they are not reconstructed
  from the service. For Compute services, Fastly can return package metadata
  such as the source hash, but it cannot reconstruct the local package
  filename or contents. [Step 5](#5-finish-the-automatic-family-configuration)
  covers what to put back.

Compare the generated configuration against your existing one, even if you
plan to delete this directory afterward. The generated file is the beta
provider's own reading of your service. Some differences are expected because
generated configuration expands defaults and replaces Terraform expressions
with literal values. Unexpected semantic differences may indicate a bug or an
undocumented behavior change and are worth [reporting](#send-us-feedback).

### 5. Finish the Automatic-family configuration

Keep every `import` block. Terraform still needs those blocks to adopt the
existing resources into this new state. Remove only the temporary `provider`
argument from each import block:

```hcl
import {
  to = fastly_service_cdn_auto.example
  id = "<service-id>"
}
```

For a Compute service, also add the `package` block and point it at the package
you want Terraform to upload and manage:

```hcl
package {
  filename = "./package.tar.gz"
}
```

~> **Important:** `terraform validate` passes on a Compute configuration with no `package` block, so it will not catch a missing package for you. Check for it yourself.

Leave `force_destroy` at its default `false` unless you intentionally need
different deletion behavior. This guide never requires `terraform destroy`.

The rest only matters if you intend to keep the service on the beta provider.
A test directory you're going to delete works without them:

- **Replace literal IDs with references.** Generated resources refer to each
  other by ID string — a `fastly_domain` arrives with
  `service_id = "<service-id>"` rather than a reference to the service
  resource.
- **Restore variables, locals, and module structure** from your existing
  configuration. Generation writes literal values throughout, so a
  configuration you built from modules comes back flat.

### 6. Validate and plan the Automatic-family import

Format and validate the generated configuration, then create a fresh plan:

```bash
terraform fmt
terraform validate
terraform plan
```

At this point the working directory still has no imported Terraform state. The
`resource` blocks describe what Terraform should manage, while the `import`
blocks tell Terraform that those resources already exist and must be adopted
instead of created.

For an existing service, the plan must show an **import**, not a create. A
service-only example begins with output similar to:

```text
fastly_service_cdn_auto.example: Preparing import... [id=<service-id>]
fastly_service_cdn_auto.example: Refreshing state... [id=<service-id>]
```

If Terraform instead proposes `+ create` for a resource that already exists on
Fastly, **do not apply the plan**. Check that the matching `import` block is
still present, that its `to` address exactly matches the generated resource
address, and that its `id` identifies the existing Fastly resource.

Terraform may still report an in-place update even when the import will not
change anything in the Fastly API. In particular, `force_destroy` and `reuse`
exist only in Terraform and have no corresponding API values. Because they
cannot be recovered during import, Terraform plans to record the configured
value, or the default `false`, in state:

```text
+ force_destroy = false
+ reuse         = false
```

These are Terraform-side lifecycle settings and do not cause a Fastly API
update. `active_version` and `managed_version` may also be recomputed during
the import.

A normal service-only import can therefore end with a summary such as:

```text
Plan: 1 to import, 0 to add, 1 to change, 0 to destroy.
```

The important safety signals are that the existing service is being imported,
nothing is being added, and nothing is being destroyed. Review any additional
changes carefully. Do not continue if Terraform proposes creating the existing
service, destroying resources, or making other unexpected remote changes.

### 7. Apply the Automatic-family import

This is the step that writes the import into Terraform state and makes the
beta configuration responsible for the service. It is also the first step that
may change Fastly. Use a **non-production** service. If it's a Compute service,
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

### 8. Exercise the Automatic-family workflow

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
has a longer list of suggested tests with expected outcomes.

### 9. When you're done testing the Automatic family

**If you stopped before applying**, nothing on Fastly changed. Delete the
directory.

**If you applied**, take stock of three things before you decide what to do
with the service:

- The beta provider's state now manages the service.
- The service may be running a version the beta provider activated. Taking
  over a Compute service always produces one. A CDN service only gets one if
  you made changes in [step 8](#8-exercise-the-automatic-family-workflow).
- Your legacy state may now be stale, especially for a Compute service or
  after changes made in [step 8](#8-exercise-the-automatic-family-workflow).

**To hand the service back to the legacy provider**, work in your existing
configuration's directory:

```bash
# in your existing configuration's directory
terraform plan
terraform apply
```

The plan shows how the legacy provider would reconcile the resources it
manages back to the legacy configuration. Read it carefully before you apply
rather than assuming it is a rollback. For a Compute service it may re-upload
the package your legacy configuration points at, and changes made during
testing may produce additional reconciliation.

Then delete the beta directory.

~> **Important:** Delete the beta directory. Do not run `terraform destroy` in it — the beta provider manages the real service, so a destroy deletes the service itself, not just your test setup.

**To keep the service on the beta provider**, remove **every** object you
imported from the legacy state, not just the service, and delete their resource
blocks from that configuration:

```bash
# in your existing configuration's directory
terraform state rm -dry-run fastly_service_vcl.example
terraform state rm fastly_service_vcl.example
terraform state rm fastly_domain.example
```

`state rm` removes only the resources you name, and changes nothing in Fastly.
Delete the resource blocks too, along with anything that references them —
otherwise the next legacy plan will try to create the service again.

## Test the Explicit family

Use this path when you want service configuration represented as first-class
resources and you want cloning, validation, staging, and activation to remain
explicit operations outside normal resource CRUD.

This workflow uses Terraform's `list` blocks and `terraform query` to discover
existing Fastly resources and generate both resource configuration and
identity-based import blocks.

The example below uses a CDN service. For a Compute service, use
`fastly_service_compute` for the service resource and query only resource types
supported by your service.

### 1. Create an Explicit-family working directory

Create a new, empty directory with its own Terraform state.

This workflow uses the beta provider published on the public Terraform
Registry at `fastly/fastly-beta`.

For this workflow, use `fastly-beta` as the **local Terraform provider name**
exactly as shown below. This does not change where Terraform installs the
provider from: `source = "fastly/fastly-beta"` still installs the published
provider from the Registry. The local name is intentionally `fastly-beta`
because `terraform query -generate-config-out` writes
`provider = fastly-beta` into generated resource and import blocks. Matching
that name lets the generated configuration validate without manual
provider-reference edits.

Create `main.tf`:

```hcl
terraform {
  required_providers {
    fastly-beta = {
      source  = "fastly/fastly-beta"
      version = "<current-version>"
    }
  }
}

provider "fastly-beta" {}
```

Replace `<current-version>` with the current published version from the
[Terraform Registry](https://registry.terraform.io/providers/fastly/fastly-beta/latest).
Pin the version and keep the `.terraform.lock.hcl` file that `terraform init`
writes so that a test can be reproduced against the same beta release.

For a focused test, use an API token that can access only the non-production
service you want to adopt. List resources enumerate matching resources visible
to the token, so limiting the token prevents an accidental generated
configuration containing other services.

```bash
export FASTLY_API_TOKEN=<service-scoped-token>
terraform init
```

`terraform init` downloads the pinned `fastly/fastly-beta` release from the
Terraform Registry.

### 2. Define the resources to query

Create `fastly.tfquery.hcl`. Each `list` block asks the provider to discover
one explicit resource type.

For example, a CDN service with domains, backends, service settings, and gzip
configuration can start with:

```hcl
list "fastly_service_cdn" "all" {
  provider = fastly-beta
}

list "fastly_service_domain" "all" {
  provider = fastly-beta
}

list "fastly_service_backend" "all" {
  provider = fastly-beta
}

list "fastly_service_settings" "all" {
  provider = fastly-beta
}

list "fastly_service_gzip" "all" {
  provider = fastly-beta
}
```

Add a `list` block for every explicit resource type you want Terraform to
discover. Query only returns resource types that you ask for. The
[`terraform query` support
documentation](https://github.com/fastly/terraform-provider-fastly-beta/blob/main/docs/terraform-query.md)
lists all supported explicit resources and their list blocks.

It is normal for a list block to return zero results when the service does not
contain that resource type.

### 3. Discover the existing service

Run:

```bash
terraform query
```

This operation is read-only. It discovers resources but does not create
Terraform state and does not change the Fastly service.

For a service-scoped token, the output should contain only resources belonging
to the service you are testing. For example:

```text
list.fastly_service_cdn.all       service_id=<service-id>                  <display-name>
list.fastly_service_domain.all    name=<domain>,service_id=<service-id>    <display-name>
list.fastly_service_backend.all   name=<backend>,service_id=<service-id>   <display-name>
list.fastly_service_settings.all  service_id=<service-id>                  <display-name>
```

Query reads first-class versioned resources from the **active service version**
when one exists. If the service has no active version, it reads the latest
version instead.

### 4. Generate the Explicit-family configuration

Generate resource and identity-based import blocks:

```bash
terraform query -generate-config-out=generated.tf
```

The output path must not already exist. Remove `generated.tf` before
regenerating it:

```bash
rm -f generated.tf
terraform query -generate-config-out=generated.tf
```

Generation is still read-only. Nothing is imported until you run a Terraform
plan and apply the generated import blocks.

### 5. Review the Explicit-family configuration

Read `generated.tf` before planning an import.

Versioned explicit resources contain the service version that query read. The
version belongs to normal resource configuration, but it is deliberately not
part of persistent Terraform resource identity.

For example:

```hcl
resource "fastly_service_backend" "all_0" {
  provider   = fastly-beta
  service_id = "<service-id>"
  version    = 2
  name       = "origin"
  address    = "origin.example.com"
  # ...
}

import {
  to       = fastly_service_backend.all_0
  provider = fastly-beta
  identity = {
    name       = "origin"
    service_id = "<service-id>"
  }
}
```

The important distinction is:

```text
resource configuration: service_id + version + name
resource identity:      service_id + name
```

The mutable service version must not appear in the identity. This lets the
same logical explicit resource move from one service version to another
without changing Terraform identity.

Service resources use `service_id` as identity. Most named versioned resources
use `service_id + name`. Service settings use `service_id`, and CDN ACL entry
collections use `service_id + acl_id`.

Review the generated file for:

- only the service and resource types you intended to discover
- the expected active/latest service version
- `version` in versioned resource configuration, but not in import identity
- provider references using `fastly-beta`
- any defaults or literal values you want to simplify before keeping the
  configuration long term

Generated configuration is a snapshot. If the active service version changes
between generation and import, regenerate the file or review the version
arguments carefully before continuing.

### 6. Validate and plan the Explicit-family import

Format and validate the generated configuration:

```bash
terraform fmt
terraform validate
```

Then create a saved plan:

```bash
terraform plan -out=query-import.tfplan
terraform show query-import.tfplan
```

Terraform should prepare **imports**, not creates, for existing resources. A
backend import looks similar to:

```text
fastly_service_backend.all_0: Preparing import... [identity=name=origin,service_id=<service-id>]
fastly_service_backend.all_0: Refreshing state...
```

A healthy import plan has no unexpected creates, destroys, or replacements.
For example:

```text
Plan: 4 to import, 0 to add, 1 to change, 0 to destroy.
```

An in-place change on the service can be expected when Terraform records
provider-only fields such as `force_destroy = false` or `reuse = false`.
Review every reported change before continuing.

If Terraform proposes creating a resource that already exists, destroying
resources, or replacing existing resources unexpectedly, **do not apply the
plan**.

### 7. Apply the Explicit-family import

Apply the saved plan:

```bash
terraform apply query-import.tfplan
```

Terraform imports the discovered service and explicit resources into the new
state using the generated resource identities.

List the imported resources:

```bash
terraform state list
```

Then immediately verify convergence:

```bash
terraform plan
```

The expected result is:

```text
No changes. Your infrastructure matches the configuration.
```

If the follow-up plan reports changes, review them before using this
configuration for further service changes. A no-op follow-up plan is the
strongest confirmation that discovery, generated configuration, identity-based
import, refresh, and state all agree.

~> **Important:** While you're testing, don't run another Terraform configuration that manages the same Fastly objects. Two states managing the same resources will fight each other.

### 8. Continue with caller-managed service versions

The generated explicit resources point to the active version that query read
(or the latest version when no active version exists). An active or locked
version is suitable for discovery and import, but it is not a writable target
for subsequent configuration changes.

Before changing explicit resources:

1. clone or select a writable Fastly service version
2. update the explicit resources to target that version
3. run Terraform plan and apply against the writable version
4. validate, stage, or activate the version explicitly when you are ready

Normal CRUD for the Explicit family does **not** clone or activate service
versions for you. Manage that lifecycle with the Fastly CLI, Fastly API, or
Terraform Actions, depending on your workflow.

The repository contains complete orchestration examples rather than duplicating
those lifecycle procedures here:

- [`orchestration-explicit-cli`](https://github.com/fastly/terraform-provider-fastly-beta/tree/main/examples/orchestration-explicit-cli)
  demonstrates explicit cloning and activation with the Fastly CLI.
- [`orchestration-explicit-actions`](https://github.com/fastly/terraform-provider-fastly-beta/tree/main/examples/orchestration-explicit-actions)
  demonstrates cloning, staging, and activation with Terraform Actions.
- [`compute-explicit-package`](https://github.com/fastly/terraform-provider-fastly-beta/tree/main/examples/compute-explicit-package)
  demonstrates package upload and activation for an explicit Compute service.

### 9. When you're done testing the Explicit family

**If you stopped before applying**, query and configuration generation changed
nothing on Fastly. Delete the test directory.

**If you imported the resources but made no subsequent service changes**, a
successful no-op plan confirms the generated configuration matches the live
service. If you do not want to keep the beta configuration, delete the test
directory rather than running `terraform destroy`.

**If you made and activated changes with the Explicit family**, treat any
legacy Terraform state as potentially stale. Review the legacy plan carefully
before applying it again; it may try to reconcile the service back to the
legacy configuration.

**To keep the service on the beta provider**, make sure no other Terraform
state continues to manage the same service or child resources. Remove the
overlapping objects from the old state only after you have reviewed the new
configuration and decided that the beta state should become authoritative.

~> **Important:** Do not run `terraform destroy` merely to stop testing. The imported resources represent the real Fastly service and its configuration.

### Send us feedback

The provider is still in development, which is the useful part: a problem you
report now can still be fixed by changing the design. Once the provider is
released, the same fix may introduce a breaking change for everyone already
using it, and the bar for making it is much higher.

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
