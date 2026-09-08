## [UNRELEASED]

### BREAKING:

### ENHANCEMENTS:

### BUG FIXES:

- fix(docs): add a note to the provider index page clarifying that the `-beta` suffix applies only to the Registry distribution, not to resource or data source type names ([#76](https://github.com/fastly/terraform-provider-fastly-beta/pull/76))

### Dependencies

## 0.1.0 (September 8, 2026)

Initial beta release. Introduces the rewrite of the Fastly Terraform provider on the Plugin Framework, including:

- Automatic (_auto) service resources: nested config blocks with provider-managed version lifecycle (auto-clone, validate, activate on every CRUD).
- Service building blocks: domain, backend, ACL + ACL entries, config store + items, secret store, dictionary items, cache settings, response object, request setting, custom dashboard, syslog logging, DNS zone.
- TLS: certificates, private keys, mutual authentication, activation, subscriptions, and Platform TLS.
- Next-Gen WAF (NGWAF): account rules, workspaces, workspace rules/thresholds/lists/signals/redactions/alert integrations, virtual patching.
