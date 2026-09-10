## [UNRELEASED]

### BREAKING:

### ENHANCEMENTS:

- feat(grafana_cloud_logs): add support for Grafana Cloud Logs ([#92](https://github.com/fastly/terraform-provider-fastly-beta/pull/91))

### BUG FIXES:

### Dependencies:

## 0.1.3 (September 8, 2026)

### ENHANCEMENTS:

- feat(logging_cloudfiles): add support for Cloudfiles Logging ([#85](https://github.com/fastly/terraform-provider-fastly-beta/pull/85))
- feat(rtsig_key): add `fastly_tsig_key` resource and `fastly_tsig_keys` data source ([#83](https://github.com/fastly/terraform-provider-fastly-beta/pull/83))
- feat(object_storage_access_keys): add resource for managing Fastly object storage access keys ([#84](https://github.com/fastly/terraform-provider-fastly-beta/pull/84))

### BUG FIXES:

- fix(logging): reject explicit empty string on defaulted format-like attributes (`format`, `timestamp_format`, `region`, `domain`) on Optional+Computed logging attributes, which previously bypassed the schema default and caused "Provider produced inconsistent result after apply" ([#86](https://github.com/fastly/terraform-provider-fastly-beta/pull/86))

## 0.1.2 (September 8, 2026)

### BUG FIXES:

- resource/fastly_service_cdn_auto, resource/fastly_service_dynamic_vcl_snippet: add an optional `content` attribute to dynamic VCL snippet metadata (the `dynamic_snippet` block, and the standalone resource), seeded into the snippet on creation so a service whose main VCL `include`s a dynamic snippet can be created in one apply ([#78](https://github.com/fastly/terraform-provider-fastly-beta/pull/78)).

## 0.1.1 (September 8, 2026)

### BUG FIXES:

- fix(docs): add a note to the provider index page clarifying that the `-beta` suffix applies only to the Registry distribution, not to resource or data source type names ([#76](https://github.com/fastly/terraform-provider-fastly-beta/pull/76))

## 0.1.0 (September 8, 2026)

Initial beta release. Introduces the rewrite of the Fastly Terraform provider on the Plugin Framework, including:

- Automatic (_auto) service resources: nested config blocks with provider-managed version lifecycle (auto-clone, validate, activate on every CRUD).
- Service building blocks: domain, backend, ACL + ACL entries, config store + items, secret store, dictionary items, cache settings, response object, request setting, custom dashboard, syslog logging, DNS zone.
- TLS: certificates, private keys, mutual authentication, activation, subscriptions, and Platform TLS.
- Next-Gen WAF (NGWAF): account rules, workspaces, workspace rules/thresholds/lists/signals/redactions/alert integrations, virtual patching.
