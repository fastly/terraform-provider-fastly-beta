# Service Dictionary Items Example

This example demonstrates managing the contents of a Fastly Dictionary using the `fastly_service_dictionary_items` resource, alongside a `fastly_service_cdn_auto` service that declares the dictionary.

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

- Declares a Dictionary via the `dictionary` block on `fastly_service_cdn_auto`
- Manages the Dictionary's key-value items with `fastly_service_dictionary_items`

## Important Notes

- Each key declared in `items` is owned by this resource: Terraform creates missing keys, updates keys that drift, and deletes a key when it is removed from `items`
- Items not declared in `items` are left unchanged, so a Dictionary can contain Terraform-managed items alongside items managed by other systems (the Fastly API, the Fastly CLI, or the control panel)
- `write_only` dictionaries are not supported, since their items cannot be read back to detect drift
- Import with `terraform import fastly_service_dictionary_items.example <service_id>/<dictionary_id>`
