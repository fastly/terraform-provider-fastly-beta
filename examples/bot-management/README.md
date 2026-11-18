# Bot Management (ContentGuard)

Configures ContentGuard on a CDN service: the workspace protection mode, a
category action, and a per-bot override.

> **Note:** A Bot Management workspace isn't a Next-Gen WAF workspace. Its ID
> can't be used with `fastly_ngwaf_*` resources.

## Usage

A Bot Management workspace is created when ContentGuard is enabled on a
service.

> [!WARNING]
> Do not enable ContentGuard and manage its Bot Management workspace in the
> same `apply`. Otherwise, the `fastly_bot_management_workspace` data source can
> look up the workspace before it exists, and the apply fails with a "Bot
> Management workspace not found" error. Enable ContentGuard first, then apply
> the rest of the configuration:

```shell
terraform apply -target=fastly_service_product_bot_management.example
terraform apply
```

To reset every bot in a given category back to its inherited category default:

```shell
terraform apply -invoke=action.fastly_bot_management_category_reset_bots.ai_crawler
```

## Destroying

- `fastly_bot_management_workspace` and `fastly_bot_management_category`
  are only removed from state. Their settings are left unchanged.
- `fastly_bot_management_bot` sets the bot's action back to `inherit`, so it
  follows its category's action.
