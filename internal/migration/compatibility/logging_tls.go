package compatibility

import (
	"slices"
	"strings"
)

// loggingTLSFields lists the reviewed flat source fields for the five
// legacy logging endpoints with TLS and authentication transformations.
// The published SDKv2 VCL and Compute schemas differ only in the four VCL
// fields below. In particular, Splunk request limits are target-only.
var loggingTLSFields = map[string][]string{
	"logging_https":         {"url", "compression_codec", "content_type", "gzip_level", "header_name", "header_value", "json_format", "message_type", "method", "period", "request_max_bytes", "request_max_entries"},
	"logging_elasticsearch": {"index", "url", "user", "password", "pipeline", "request_max_bytes", "request_max_entries"},
	"logging_kafka":         {"auth_method", "brokers", "compression_codec", "parse_log_keyvals", "password", "request_max_bytes", "required_acks", "topic", "use_tls", "user"},
	"logging_splunk":        {"token", "url", "use_tls"},
	"logging_syslog":        {"address", "message_type", "port", "token", "use_tls"},
}

// IsTLSLoggingBlock identifies the five reviewed endpoints. Other logging
// endpoints must be enabled by their own compatibility work packages.
func IsTLSLoggingBlock(block string) bool {
	_, ok := loggingTLSFields[block]
	return ok
}

// TLSLoggingLegacyField reports whether a field exists in the reviewed flat
// source schema. Matching a target field alone does not make a source valid.
func TLSLoggingLegacyField(block, field string, vcl bool) bool {
	fields, ok := loggingTLSFields[block]
	if !ok {
		return false
	}
	if slices.Contains(fields, field) || slices.Contains([]string{"name", "processing_region", "tls_ca_cert", "tls_client_cert", "tls_client_key", "tls_hostname"}, field) {
		return true
	}
	return vcl && slices.Contains([]string{"format", "format_version", "placement", "response_condition"}, field)
}

// TLSLoggingObjectRoute returns the exact object and member for a reviewed
// flat credential route, independent of the resource family. The compiler
// additionally requires an automatic registry rule with this exact target.
func TLSLoggingObjectRoute(block, field string) (object, member string, ok bool) {
	if !IsTLSLoggingBlock(block) {
		return "", "", false
	}
	switch field {
	case "tls_ca_cert", "tls_client_cert", "tls_client_key", "tls_hostname":
		return "tls", strings.TrimPrefix(field, "tls_"), true
	case "user", "password":
		if block == "logging_elasticsearch" || block == "logging_kafka" {
			return "authentication", field, true
		}
	case "token":
		if block == "logging_splunk" || block == "logging_syslog" {
			return "authentication", field, true
		}
	}
	return "", "", false
}
