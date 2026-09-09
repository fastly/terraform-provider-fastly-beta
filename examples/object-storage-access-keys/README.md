# Object Storage Access Keys Example

Demonstrates creating a `fastly_object_storage_access_keys` resource.

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

## Notes

- The resource is versionless and immutable: changing `description`, `permission`, or `buckets` destroys and recreates it.
- `authentication.secret_key` is only returned at creation time and is not populated by import or later reads.
