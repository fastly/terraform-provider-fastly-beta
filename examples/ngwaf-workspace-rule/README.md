# NGWAF Workspace Rule Example

This example demonstrates managing Fastly Next-Gen WAF rules scoped to a
single workspace using the `fastly_ngwaf_workspace_rule` resource, alongside
the `fastly_ngwaf_workspace_rules` data source.

## Usage

1. Set your Fastly API token:
   ```bash
   export FASTLY_API_TOKEN=your_token_here
   ```

2. Initialize Terraform:
   ```bash
   terraform init
   ```

3. Apply the configuration:
   ```bash
   terraform apply
   ```

## Features

- Creates a workspace, independent of any service or service version
- Manages a simple `request`-type rule with a single flat `condition`
- Manages a `request`-type rule combining a `group_condition` and a nested
  `multival_condition`
- Manages a `request`-type rule using a standalone, top-level
  `multival_condition`
- Manages a `signal`-type rule that excludes a false-positive signal
- Manages a `rate_limit`-type rule
- Manages a `templated_signal`-type rule
- Reads back all rules in the workspace via `fastly_ngwaf_workspace_rules`

## Important Notes

- `fastly_ngwaf_workspace_rule` is versionless: it is not tied to a service
  version, only to the workspace it belongs to
- A rule must define at least one of `condition`, `group_condition`, or
  `multival_condition`
