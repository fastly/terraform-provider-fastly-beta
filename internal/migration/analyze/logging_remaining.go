package analyze

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
)

// analyzeRemainingCDNLoggingBlock rejects unreviewed source attributes and
// statically provable invalid configurations before generating HCL. Values
// of credentials (even literal values) must never appear in findings.
func analyzeRemainingCDNLoggingBlock(state *analysisState, block *hclsyntax.Block) {
	attrs := block.Body.Attributes
	reject := func(rng hcl.Range, name, instruction string) {
		state.result.Findings = append(state.result.Findings, Finding{
			Severity: SeverityError, Range: rng,
			Message:     fmt.Sprintf("Legacy %s option %q cannot be safely migrated.", block.Type, name),
			Remediation: instruction,
		})
	}
	known := func(name string) (cty.Value, bool) {
		attr := attrs[name]
		if attr == nil {
			return cty.NilVal, false
		}
		value, diags := attr.Expr.Value(nil)
		return value, !diags.HasErrors() && value.IsKnown()
	}
	present := func(name string) bool {
		if attrs[name] == nil {
			return false
		}
		value, ok := known(name)
		return !ok || !value.IsNull()
	}
	for name, attr := range attrs {
		if !compatibility.RemainingCDNLoggingLegacyField(block.Type, name) {
			reject(attr.NameRange, name, "Use the published SDKv2 flat endpoint attributes; target-only nested objects are not supported as legacy inputs.")
		}
	}
	// Only required fields that cannot be recovered from target defaults.
	required := map[string][]string{
		"logging_datadog":          {"name", "token"},
		"logging_ftp":              {"name", "address", "user", "password", "path"},
		"logging_sftp":             {"name", "address", "user", "path", "ssh_known_hosts"},
		"logging_loggly":           {"name", "token"},
		"logging_grafanacloudlogs": {"name", "user", "url", "index", "token"},
		"logging_scalyr":           {"name", "token"},
		"logging_newrelic":         {"name", "token"},
		"logging_newrelicotlp":     {"name", "token"},
		"logging_heroku":           {"name", "token", "url"},
		"logging_honeycomb":        {"name", "token", "dataset"},
		"logging_logshuttle":       {"name", "token", "url"},
		"logging_papertrail":       {"name", "address", "port"},
		"logging_sumologic":        {"name", "url"},
		"logging_fos":              {"name", "bucket_name", "region"},
	}
	for _, name := range required[block.Type] {
		if attrs[name] == nil {
			reject(block.TypeRange, name, "Supply the required legacy endpoint field before migration.")
			continue
		}
		if value, ok := known(name); ok && (value.IsNull() ||
			(value.Type() == cty.String && strings.TrimSpace(value.AsString()) == "")) {
			reject(attrs[name].NameRange, name, "Required values cannot be null or blank.")
		}
	}
	if block.Type == "logging_sftp" {
		// Exactly one credential mechanism must be specified. An expression
		// is treated as configured unless it is statically known to be null.
		p, k := present("password"), present("secret_key")
		if p == k {
			reject(block.TypeRange, "password/secret_key", "Configure exactly one SFTP password or SSH private key; ambiguous configurations are blocked.")
		}
	}
	for _, name := range []string{"format", "format_version", "placement", "processing_region", "region", "message_type", "compression_codec", "gzip_level"} {
		value, ok := known(name)
		if !ok || value.IsNull() {
			continue
		}
		switch name {
		case "format":
			if value.Type() != cty.String || strings.TrimSpace(value.AsString()) == "" {
				reject(attrs[name].NameRange, name, "Use a nonempty VCL format or omit it to preserve the exact SDKv2 default.")
			}
		case "placement":
			if value.Type() != cty.String || value.AsString() != "none" {
				reject(attrs[name].NameRange, name, `Only statically known placement = "none" is verified; expressions are checked by the target provider.`)
			}
		case "format_version":
			if value.Type() != cty.Number || !value.AsBigFloat().IsInt() ||
				value.LessThan(cty.NumberIntVal(1)).True() || value.GreaterThan(cty.NumberIntVal(2)).True() {
				reject(attrs[name].NameRange, name, "Only VCL log format versions 1 and 2 are supported.")
			}
		case "processing_region":
			if value.Type() != cty.String || !slices.Contains([]string{"none", "us", "eu"}, value.AsString()) {
				reject(attrs[name].NameRange, name, "Valid processing regions are none, us and eu.")
			}
		case "region":
			var allowed []string
			switch block.Type {
			case "logging_datadog":
				allowed = []string{"US", "US3", "US5", "EU", "EU1", "AP1"}
			case "logging_scalyr", "logging_newrelic":
				allowed = []string{"US", "EU"}
			}
			if len(allowed) > 0 && (value.Type() != cty.String || !slices.Contains(allowed, value.AsString())) {
				reject(attrs[name].NameRange, name, "Use a region supported by the target logging endpoint.")
			}
		case "message_type":
			if value.Type() != cty.String || !slices.Contains([]string{"classic", "loggly", "logplex", "blank"}, value.AsString()) {
				reject(attrs[name].NameRange, name, "Use a message type supported by the target logging endpoint.")
			}
		case "compression_codec":
			if value.Type() != cty.String || !slices.Contains([]string{"gzip", "snappy", "zstd"}, value.AsString()) {
				reject(attrs[name].NameRange, name, "Use a compression codec supported by the Framework logging endpoint.")
			}
		case "gzip_level":
			if value.Type() != cty.Number || !value.AsBigFloat().IsInt() ||
				value.LessThan(cty.NumberIntVal(-1)).True() || value.GreaterThan(cty.NumberIntVal(9)).True() {
				reject(attrs[name].NameRange, name, "Use the legacy unset sentinel -1 or a gzip level between 0 and 9.")
			}
		}
	}
	if attrs["compression_codec"] != nil && attrs["gzip_level"] != nil &&
		present("compression_codec") && present("gzip_level") {
		gzip, gzipOK := known("gzip_level")
		// Explicit -1 represents the unset SDKv2 default, not a competing
		// configured compression choice.
		isUnset := gzipOK && !gzip.IsNull() && gzip.Type() == cty.Number &&
			gzip.RawEquals(cty.NumberIntVal(-1))
		if !isUnset {
			reject(attrs["gzip_level"].NameRange, "gzip_level",
				"Choose either a configured gzip_level or compression_codec, not both.")
		}
	}
}
