---
page_title: "fastly_tls_configuration Data Source - fastly"
subcategory: ""
description: |-
  Get information on a Fastly TLS configuration.
---

# fastly_tls_configuration (Data Source)

Use this data source to get the ID of a TLS configuration for use with other resources.

The data source's filters are applied using an **AND** boolean operator, so depending on the combination
of filters, they may become mutually exclusive. The exception to this is `id`, which must not be specified
in combination with any of the others.

If more or less than a single match is returned by the search, an error is raised. Ensure that your search
is specific enough to return a single result.

## Example Usage

```terraform
data "fastly_tls_configuration" "example" {
  default = true
}

output "tls_configuration_id" {
  value = data.fastly_tls_configuration.example.id
}
```

## Schema

### Optional

- `default` (Boolean) Signifies whether Fastly will use this configuration as a default when creating a new TLS activation.
- `http_protocols` (Set of String) HTTP protocols available on the TLS configuration.
- `id` (String) ID of the TLS configuration obtained from the Fastly API or another data source. Conflicts with all the other filters.
- `name` (String) Custom name of the TLS configuration.
- `tls_protocols` (Set of String) TLS protocols available on the TLS configuration.
- `tls_service` (String) Whether the configuration should support the `PLATFORM` or `CUSTOM` TLS service.

### Read-Only

- `created_at` (String) Timestamp (GMT) when the configuration was created.
- `dns_records` (Attributes Set) The available DNS addresses that can be used to enable TLS for a domain. DNS must be configured for a domain for TLS handshakes to succeed. If enabling TLS on an apex domain (e.g. `example.com`) you must create four A records (or four AAAA records for IPv6 support) using the displayed global A record's IP addresses with your DNS provider. For subdomains and wildcard domains (e.g. `www.example.com` or `*.example.com`) you will need to create a relevant CNAME record. (see [below for nested schema](#nestedatt--dns_records))
- `updated_at` (String) Timestamp (GMT) when the configuration was last updated.

<a id="nestedatt--dns_records"></a>
### Nested Schema for `dns_records`

Read-Only:

- `record_type` (String) Type of DNS record to set, e.g. A, AAAA, or CNAME.
- `record_value` (String) The IP address or hostname of the DNS record.
- `region` (String) The regions that will be used to route traffic. Select DNS records with a `global` region to route traffic to the most performant point of presence (POP) worldwide (global pricing will apply). Select DNS records with a `us-eu` region to exclusively land traffic on North American and European POPs.
