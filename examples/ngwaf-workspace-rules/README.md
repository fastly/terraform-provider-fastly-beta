# NGWAF Workspace Rules Example

This example demonstrates managing Fastly Next-Gen WAF rules scoped to a
single workspace. Each rule type has its own resource —
`fastly_ngwaf_workspace_request_rule`,
`fastly_ngwaf_workspace_signal_rule`,
`fastly_ngwaf_workspace_rate_limit_rule`, and
`fastly_ngwaf_workspace_templated_signal_rule` — alongside the
`fastly_ngwaf_workspace_rules` data source, which reads back every rule in
the workspace regardless of type.

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
- Manages a request rule with a single flat `condition`
- Manages a request rule combining a `group_condition` and a nested
  `multival_condition`
- Manages a request rule using a standalone, top-level `multival_condition`
- Manages a signal rule that excludes a false-positive signal
- Manages a rate limit rule
- Manages a templated signal rule
- Reads back all rules in the workspace via `fastly_ngwaf_workspace_rules`

## Important Notes

- Workspace rule resources are versionless: they are not tied to a service
  version, only to the workspace they belong to
- Every rule must define at least one of `condition`, `group_condition`, or
  `multival_condition`
- Changing any attribute of a templated signal rule replaces it
