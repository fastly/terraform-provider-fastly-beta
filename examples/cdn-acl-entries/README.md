# CDN ACL Entries Example

This example demonstrates managing ACL entries for a Fastly service using the `fastly_service_cdn_acl_entries` resource.

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

- Creates a VCL service with a domain and backend
- Creates an ACL on version 1 of the service
- Manages multiple ACL entries with different configurations:
  - Single IP addresses
  - IP ranges (using CIDR notation)
  - Negated entries (blocks instead of allows)

## Important Notes

- Terraform manages only the entries declared in `entry` blocks; entries created outside Terraform (via the API, CLI, or control panel) are left untouched and ignored for drift
- Removing an `entry` block from configuration deletes that entry on the next apply
- ACL entries are created in batches for performance
- Entry IDs are computed and managed by the Fastly API
