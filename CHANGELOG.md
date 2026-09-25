## [UNRELEASED]

### BREAKING:

- resource/fastly_service_vcl, resource/fastly_service_cdn_auto: renamed `fastly_service_vcl` to `fastly_service_custom_vcl`, and its corresponding nested `vcl` block on `fastly_service_cdn_auto` to `custom_vcl` ([#127](https://github.com/fastly/terraform-provider-fastly-beta/pull/127))

### ENHANCEMENTS:

- feat(docs): add a beta testing guide, and provider overview content explaining the resource families ([#143](https://github.com/fastly/terraform-provider-fastly-beta/pull/143))
- feat(grafana_cloud_logs): add support for Grafana Cloud Logs ([#92](https://github.com/fastly/terraform-provider-fastly-beta/pull/92))
- feat(logging_grafanacloudlogs): add support for Grafana Cloud Logs, including the explicit `fastly_service_logging_grafanacloudlogs` resource ([#92](https://github.com/fastly/terraform-provider-fastly-beta/pull/92))
- feat(ngwaf_workspace_templated_signal_rules): moved Templated Signal Rules to the dedicated `fastly_ngwaf_workspace_templated_signal_rules` data source ([#90](https://github.com/fastly/terraform-provider-fastly-beta/pull/90))
- feat(logging_digitalocean): add support for Digital Ocean Logging, including the explicit `fastly_service_logging_digitalocean` resource ([#95](https://github.com/fastly/terraform-provider-fastly-beta/pull/95))
- feat(api_security_discovered_operations): add `fastly_api_security_discovered_operations` data source ([#96](https://github.com/fastly/terraform-provider-fastly-beta/pull/96))
- feat(datacenters): add `fastly_datacenters` data source ([#98](https://github.com/fastly/terraform-provider-fastly-beta/pull/98))
- feat(staging_ips): add `fastly_staging_ips` data source ([#99](https://github.com/fastly/terraform-provider-fastly-beta/pull/99))
- feat(tls_domain): add `fastly_tls_domain` data source ([#100](https://github.com/fastly/terraform-provider-fastly-beta/pull/100))
- feat(api_security_operations): add `fastly_api_security_operations` and `fastly_api_security_operation_tags` data sources ([#101](https://github.com/fastly/terraform-provider-fastly-beta/pull/101))
- feat(package_hash): add `fastly_package_hash` data source ([#103](https://github.com/fastly/terraform-provider-fastly-beta/pull/103))
- feat(services): add `fastly_services` data source ([#102](https://github.com/fastly/terraform-provider-fastly-beta/pull/102))
- feat(ip_ranges): add `fastly_ip_ranges` data source ([#104](https://github.com/fastly/terraform-provider-fastly-beta/pull/104))
- feat(logging_elasticsearch): add support for Elasticsearch Logging, including the explicit `fastly_service_logging_elasticsearch` resource ([#105](https://github.com/fastly/terraform-provider-fastly-beta/pull/105))
- feat(logging_ftp): add support for FTP Logging, including the explicit `fastly_service_logging_ftp` resource ([#107](https://github.com/fastly/terraform-provider-fastly-beta/pull/107))
- feat(logging_googlepubsub): add support for Google Cloud Pub/Sub Logging, including the explicit `fastly_service_logging_googlepubsub` resource ([#106](https://github.com/fastly/terraform-provider-fastly-beta/pull/106))
- feat(service_product_ddos_protection): Add support for 'client_challenge' mode. ([#110](https://github.com/fastly/terraform-provider-fastly-beta/pull/110))
- feat(logging_heroku): add support for Heroku Logging, including the explicit `fastly_service_logging_heroku` resource ([#108](https://github.com/fastly/terraform-provider-fastly-beta/pull/108))
- feat(logging_kafka): add support for Kafka Logging, including the explicit `fastly_service_logging_kafka` resource ([#111](https://github.com/fastly/terraform-provider-fastly-beta/pull/111))
- feat(logging_honeycomb): add support for Honeycomb Logging, including the explicit `fastly_service_logging_honeycomb` resource ([#112](https://github.com/fastly/terraform-provider-fastly-beta/pull/112))
- feat(logging_logshuttle): add support for Log Shuttle Logging, including the explicit `fastly_service_logging_logshuttle` resource ([#117](https://github.com/fastly/terraform-provider-fastly-beta/pull/117))
- feat(logging_loggly): add support for Loggly Logging, including the explicit `fastly_service_logging_loggly` resource ([#113](https://github.com/fastly/terraform-provider-fastly-beta/pull/113))
- feat(logging_kinesis): add support for Kinesis Logging, including the explicit `fastly_service_logging_kinesis` resource ([#116](https://github.com/fastly/terraform-provider-fastly-beta/pull/116))
- feat(logging_scalyr): add support for Scalyr Logging, including the explicit `fastly_service_logging_scalyr` resource ([#119](https://github.com/fastly/terraform-provider-fastly-beta/pull/119))
- feat(logging_openstack): add support for OpenStack Logging, including the explicit `fastly_service_logging_openstack` resource ([#120](https://github.com/fastly/terraform-provider-fastly-beta/pull/120))
- feat(logging_sftp): add support for SFTP Logging, including the explicit `fastly_service_logging_sftp` resource ([#124](https://github.com/fastly/terraform-provider-fastly-beta/pull/124))
- feat(logging_papertrail): add support for Papertrail Logging, including the explicit `fastly_service_logging_papertrail` resource ([#123](https://github.com/fastly/terraform-provider-fastly-beta/pull/123))
- feat(tls_configuration): add `staging_ip` attribute to the `fastly_tls_configuration` data source ([#126](https://github.com/fastly/terraform-provider-fastly-beta/pull/126))
- feat(service_settings): add `fastly_service_settings` resource for explicit service version management of general settings; CDN services only ([#128](https://github.com/fastly/terraform-provider-fastly-beta/pull/128))
- feat(rate_limiter): add explicit family support for Rate Limiting ([#134](https://github.com/fastly/terraform-provider-fastly-beta/pull/134))
- feat(dictionary): add `fastly_service_dictionary` resource for explicit service version management of Edge Dictionaries; CDN and Compute services ([#136](https://github.com/fastly/terraform-provider-fastly-beta/pull/136))
- feat(cache_settings): add explicit family support for Cache Settings ([#138](https://github.com/fastly/terraform-provider-fastly-beta/pull/138))
- feat(gzip): add `fastly_service_gzip` resource for explicit service version management of Gzip configurations; CDN services only ([#142](https://github.com/fastly/terraform-provider-fastly-beta/pull/142))
- feat(dictionaries): add `fastly_dictionaries` data source, ported from the legacy provider ([#146](https://github.com/fastly/terraform-provider-fastly-beta/pull/146))
- feat(logging_bigquery): add support for BigQuery Logging, including the explicit `fastly_service_logging_bigquery` resource
- feat(logging_blobstorage): add support for Blob Storage Logging, including the explicit `fastly_service_logging_blobstorage` resource
- feat(logging_datadog): add support for Datadog Logging, including the explicit `fastly_service_logging_datadog` resource
- feat(logging_gcs): add support for GCS Logging, including the explicit `fastly_service_logging_gcs` resource
- feat(logging_https): add support for HTTPS Logging, including the explicit `fastly_service_logging_https` resource
- feat(logging_newrelic): add support for New Relic Logging, including the explicit `fastly_service_logging_newrelic` resource
- feat(logging_newrelicotlp): add support for New Relic OTLP Logging, including the explicit `fastly_service_logging_newrelicotlp` resource
- feat(logging_s3): add support for S3 Logging, including the explicit `fastly_service_logging_s3` resource
- feat(logging_splunk): add support for Splunk Logging, including the explicit `fastly_service_logging_splunk` resource
- feat(logging_sumologic): add support for Sumo Logic Logging, including the explicit `fastly_service_logging_sumologic` resource
- feat(logging_syslog): add support for Syslog Logging, including the explicit `fastly_service_logging_syslog` resource

### BUG FIXES:

- fix(cloudfiles_logging): ensure that the `format` attribute can't be set to null ([#93](https://github.com/fastly/terraform-provider-fastly-beta/pull/93))
- fix(docs): use versionless domains in examples, show the nested `domain` block as an alternative on the TLS pages, and note that classic domains are only available on accounts created before September 16, 2025 ([#121](https://github.com/fastly/terraform-provider-fastly-beta/pull/121))
- fix(docs): correct the README list of explicit resources, and map each one to its automatic-family equivalent ([#122](https://github.com/fastly/terraform-provider-fastly-beta/pull/122))
- fix(docs): add `fastly_service_settings` to the README table of explicit resources ([#132](https://github.com/fastly/terraform-provider-fastly-beta/pull/132))
- fix(docs): fill gaps and correct errors in the HCL syntax changes guide ([#131](https://github.com/fastly/terraform-provider-fastly-beta/pull/131))
- test(service_cdn_acl): add acceptance test coverage verifying the explicit `fastly_service_cdn_acl` resource rejects Compute services ([#135](https://github.com/fastly/terraform-provider-fastly-beta/pull/135))
- fix(compute): ensure that the Compute package hash is refreshed on `import` ([#140](https://github.com/fastly/terraform-provider-fastly-beta/pull/140))
- fix(gzip, condition, cache_settings): add `RequiresReplace` to `name` and `service_id` on the explicit `fastly_service_gzip`, `fastly_service_condition`, and `fastly_service_cache_setting` resources; changing either previously called Update against a name-keyed path that no longer matched the API's actual object, rather than the destroy/recreate their docs already described

### Dependencies:

## 0.1.3 (September 8, 2026)

### ENHANCEMENTS:

- feat(logging_cloudfiles): add support for Cloudfiles Logging, including the explicit `fastly_service_logging_cloudfiles` resource ([#85](https://github.com/fastly/terraform-provider-fastly-beta/pull/85))
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
