---
page_title: HCL Syntax Changes
subcategory: "Guides"
---

## HCL Syntax Changes from the 'legacy' Fastly Terraform provider

This provider incorporates a number of syntax changes which resolve
long-standing customer requests or issues, but are not backwards
compatible with the legacy Fastly Terraform provider.

**NOTE**: This guide is not a _migration_ guide; migration involves
both changes to the HCL and migration of the Terraform state file. A
migration guide for this provider will be made available at a later
date.

If you discover a syntax change while testing this provider which is
not documented in this guide, please open a support ticket with Fastly
Support to report it so that the guide can be updated.

### Service Resources

The `fastly_service_vcl` resource has been renamed to
`fastly_service_cdn_auto`, and the `fastly_service_compute` resource
has been renamed to `fastly_service_compute_auto`. The `vcl` -> `cdn`
change reflects Fastly's product naming for CDN services, and the
`auto` suffix indicates that these resources implement the _automatic
version_ workflow.

Because these resources only support the _automatic version_ workflow,
the `activate` and `stage` attributes available in the legacy provider
are not available. The `auto` service resources will always clone,
modify, and activate a new version of the service during `terraform
apply` if the plan includes changes to the service or any of its
nested blocks.

### Product Enablement

Product enablement is now available as top-level versionless
resources, and no longer available as a nested block in a `service`
resource. This changes aligns to the Fastly product enablement API,
and also allows individual products to be enabled (or diabled) without
affecting the enablement status or configuration of other products on
the service.

In the legacy provider, enablement and configuration of Domain
Inspector and Next-Gen WAF on a CDN service was written this way:

```hcl
resource "fastly_service_vcl" "example" {
  name = "example"
  product_enablement {
    domain_inspector = true
	ngwaf {
	  enabled = true
	  workspace_id = <workspace id>
	  traffic_ramp = 100
	}
  }
}
```

In this provider, the same configuration is accomplished using three
resources:

```hcl
resource "fastly_service_cdn_auto" "example" {
  name = "example"
}

resource "fastly_service_product_domain_inspector" "example" {
  service_id = fastly_service_cdn_auto.example.id
}

resource "fastly_service_product_ngwaf" "example" {
  service_id = fastly_service_cdn_auto.example.id
  workspace_id = <workspace id>
  traffic_ramp = 100
}
```

The product enablement resources will enable their respective products
on the identified service when they exist, and disable their
respective products on the service when they are
removed. Additionally, the `enabled` attribute of a product enablement
resource can be used to explicitly enable or disable the product
without removing the resource from the HCL.

Unlike the legacy provider, adding a single product enablement
resource to the HCL for a service will not have any effect on _other_
products which are already enabled on the service; it is not necessary
to add product enablement resources to the HCL for every product used
on the service, only for those which you want to manage using
Terraform.

### Sensitive Attributes

A number of nested blocks in service resources, and attributes in
other resources, are marked _sensitive_ in the provider because they
contain information which is considered secret. In the legacy
provider, the presence of a sensitive attribute in a block meant that
`terraform plan` could not display the attributes of that block when
any of the attributes had changed, even if the sensitive attribute had
not changed.

Sensitive attributes have been moved into nested blocks with names
reflecting their usage by the resource. This allows `terraform plan`
to display the remaining (non-sensitive) attributes of the resource
when they have changed.

|Resource|Sensitive attribute block name|
|---|---|
|`fastly_integration`|`authentication`|
|`fastly_ngwaf_workspace_alert` integrations (all types)|`authentication`|
|`fastly_service_cdn_auto` - `backend` block|`ssl_client_secrets`|
|`fastly_service_cdn_auto` - `logging_https` block|`tls`|
|`fastly_service_cdn_auto` - `logging_splunk` block|`tls` and `authentication`|
|`fastly_service_cdn_auto` - `logging_syslog` block|`tls` and `authentication`|
|`fastly_service_cdn_auto` - `logging` blocks (all other types)|`authentication`|
|`fastly_service_compute_auto` - `backend` block|`ssl_client_secrets`|
|`fastly_service_compute_auto` - `logging_https` block|`tls`|
|`fastly_service_compute_auto` - `logging_splunk` block|`tls` and `authentication`|
|`fastly_service_compute_auto` - `logging_syslog` block|`tls` and `authentication`|
|`fastly_service_compute_auto` - `logging` blocks (all other types)|`authentication`|
|`fastly_tls_private_key`|`pem`|

### Container Item Management

In the legacy provider a number of resources which manage items in
containers offered `manage_entries` or `manage_items` attributes which
were difficult to understand and often led to confusion. These
attributes are no longer available. Each container item resource
manages only the items _explicitly_ listed in its resource block,
ignoring any other items which may be present in the container.

This change applies to `fastly_acl_entries`,
`fastly_configstore_items`, `fastly_service_cdn_acl_entries`, and
`fastly_service_dictionary_items`.

### Next-Gen WAF Rules

Next-Gen WAF rules have their own resources by rule type and scope
('account' or 'workspace'), instead of being combined into one
resource by scope and using a `type` attribute to distinguish rule
types. This change allows more thorough validation of the resource's
configuration during `terraform plan`, reducing the chances that the
rule's attributes will be rejected by the Fastly API during `terraform
apply`.

|Legacy resource and type|New resource|
|---|---|
|`fastly_ngwaf_account_rule` - `request`|`fastly_ngwaf_request_rule`|
|`fastly_ngwaf_account_rule` - `signal`|`fastly_ngwaf_signal_rule`|
|`fastly_ngwaf_workspace_rule` - `request`|`fastly_ngwaf_workspace_request_rule`|
|`fastly_ngwaf_workspace_rule` - `signal`|`fastly_ngwaf_workspace_signal_rule`|
|`fastly_ngwaf_workspace_rule` - `rate_limit`|`fastly_ngwaf_workspace_rate_limit_rule`|
|`fastly_ngwaf_workspace_rule` - `templated_signal`|`fastly_ngwaf_workspace_templated_signal_rule`|

### Next-Gen WAF Lists

Next-Gen WAF lists have their own resources by list type and scope
('account' or 'workspace'), instead of being combined into one
resource by scope and using a `type` attribute to distinguish list
types. This change allows more thoroughl validation of the resource's
configuration during `terraform plan`, reducing the chances that the
list's attributes will be rejected by the Fastly API during `terraform
apply`.

|Legacy resource and type|New resource|
|---|---|
|`fastly_ngwaf_account_list` - `ip`|`fastly_ngwaf_ip_list`|
|`fastly_ngwaf_account_list` - `string`|`fastly_ngwaf_string_list`|
|`fastly_ngwaf_account_list` - `country`|`fastly_ngwaf_country_list`|
|`fastly_ngwaf_account_list` - `wildcard`|`fastly_ngwaf_wildcard_list`|
|`fastly_ngwaf_account_list` - `signal`|`fastly_ngwaf_signal_list`|
|`fastly_ngwaf_workspace_list` - `ip`|`fastly_ngwaf_workspace_ip_list`|
|`fastly_ngwaf_workspace_list` - `string`|`fastly_ngwaf_workspace_string_list`|
|`fastly_ngwaf_workspace_list` - `country`|`fastly_ngwaf_workspace_country_list`|
|`fastly_ngwaf_workspace_list` - `wildcard`|`fastly_ngwaf_workspace_wildcard_list`|
|`fastly_ngwaf_workspace_list` - `signal`|`fastly_ngwaf_workspace_signal_list`|

### Other Changes

`fastly_ngwaf_virtual_patches` has been renamed to
`fastly_ngwaf_workspace_virtual_patch` to reflect that it manages a
single Virtual Patch and also that it applies to a workspace, not the
entire account.

All of the `fastly_ngwaf_alert_{TYPE}_integration` resources have been
renamed to `fastly_ngwaf_workspace_alert_{TYPE}_integration` to
reflect that they apply to a workspace, not the entire account.

The deprecated aliases `fastly_domain_v1` and
`fastly_domain_v1_service_link` are not available.
