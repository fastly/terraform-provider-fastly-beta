# NGWAF Account Rules Example

This example demonstrates managing Fastly Next-Gen WAF rules scoped to the
whole account. Account scope supports two rule types, each with its own
resource — `fastly_ngwaf_request_rule` and `fastly_ngwaf_signal_rule` —
alongside the `fastly_ngwaf_rules` data source, which reads back the rules
defined at account scope.

Account rules name the workspaces they apply to through `applies_to`, either
as a set of workspace IDs or as the single entry `*` for every workspace in
the account.

See `examples/ngwaf-workspace-rules` for rules managed within a workspace.

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

- Creates two workspaces, independent of any service or service version
- Manages a request rule applied to named workspaces
- Manages a request rule applied to every workspace via `applies_to = ["*"]`
- Manages a request rule with two actions, the maximum a request rule accepts
- Manages a request rule combining a `group_condition` and a nested
  `multival_condition`
- Manages a signal rule that excludes a false-positive signal account-wide
- Reads back the account-scoped rules via `fastly_ngwaf_rules`

## Important Notes

- Account rule resources are versionless: they are not tied to a service
  version
- `applies_to` is a request body field, not a path segment, so adding or
  removing a workspace updates the rule in place rather than replacing it
- Only `request` and `signal` rules exist at account scope; `rate_limit` and
  `templated_signal` are workspace-only
- Account request rules accept the `allow`, `block`, and `add_signal` actions,
  and an account-scoped `block` carries no `redirect_url` or `response_code`
- Every rule must define at least one of `condition`, `group_condition`, or
  `multival_condition`, and no more than 10 combined
- `fastly_ngwaf_rules` covers account scope; use `fastly_ngwaf_workspace_rules`
  for a single workspace's rules, including the workspace-only `rate_limit` and
  `templated_signal` types
- Rules import from a bare rule ID: `terraform import
  fastly_ngwaf_request_rule.block_ip <rule_id>`
