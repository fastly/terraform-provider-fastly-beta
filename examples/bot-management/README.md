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

To clear the bot overrides in a given category, so every bot uses its inherited
category default:

```shell
terraform apply -invoke=action.fastly_bot_management_category_overrides_clear.ai_crawler
```

## Destroying

`fastly_bot_management_workspace`, `fastly_bot_management_category` and
`fastly_bot_management_bot` are only removed from state. Their settings are
left unchanged. To return a bot to its category's action, set
`action = "inherit"` and apply before removing its resource.
