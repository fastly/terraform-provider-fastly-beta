package analyze

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
)

// Only inspect statically known non-credential options whose target
// constraints differ. Never resolve variables, read files, or print values.
func analyzeTLSLoggingBlock(state *analysisState, block *hclsyntax.Block, vcl bool) {
	reject := func(rng hcl.Range, message, remediation string) {
		state.result.Findings = append(state.result.Findings, Finding{
			Severity: SeverityError, Range: rng, Message: message, Remediation: remediation,
		})
	}
	for name, attr := range block.Body.Attributes {
		if !compatibility.TLSLoggingLegacyField(block.Type, name, vcl) {
			reject(attr.NameRange, fmt.Sprintf("Unsupported legacy field %q in %s.", name, block.Type),
				"Use the published legacy flat endpoint schema before migrating; target-only fields and mixed objects cannot be verified.")
		}
	}
	attrs := block.Body.Attributes
	required := []string{"name"}
	switch block.Type {
	case "logging_https", "logging_splunk":
		required = append(required, "url")
	case "logging_elasticsearch":
		required = append(required, "url", "index")
	case "logging_kafka":
		required = append(required, "brokers", "topic")
	case "logging_syslog":
		required = append(required, "address")
	}
	for _, name := range required {
		if attrs[name] == nil {
			reject(block.TypeRange, fmt.Sprintf("Legacy %s requires field %q.", block.Type, name),
				"Complete the required endpoint configuration before migrating.")
		}
	}
	// These values are configuration options, not authentication material.
	// Unknown expressions are retained for Terraform to validate. Do not
	// include diagnostic text or the expression value in a finding.
	known := func(name string) (cty.Value, bool) {
		a := attrs[name]
		if a == nil {
			return cty.NilVal, false
		}
		value, diagnostics := a.Expr.Value(nil)
		return value, !diagnostics.HasErrors() && value.IsKnown()
	}
	invalid := func(name, remediation string) {
		reject(attrs[name].NameRange, fmt.Sprintf("Legacy %s option %q has no verified target equivalent.", block.Type, name), remediation)
	}
	oneOf := func(name string, options ...string) {
		if v, ok := known(name); ok && !v.IsNull() && (v.Type() != cty.String || !slices.Contains(options, v.AsString())) {
			invalid(name, "Use a value accepted by the target endpoint or omit the option if it uses the default.")
		}
	}
	if vcl {
		oneOf("placement", "none")
		if v, ok := known("format"); ok {
			literal, isLiteral := attrs["format"].Expr.(*hclsyntax.LiteralValueExpr)
			literalNull := v.IsNull() && isLiteral && literal.Val.IsNull()
			if !literalNull && (v.IsNull() || v.Type() != cty.String || strings.TrimSpace(v.AsString()) == "") {
				invalid("format", "Omit format to pin the legacy default, or configure a non-blank log format.")
			}
		}
	}
	switch block.Type {
	case "logging_https":
		oneOf("compression_codec", "gzip", "snappy", "zstd")
		if v, ok := known("gzip_level"); ok && !v.IsNull() {
			if v.Type() != cty.Number || !v.AsBigFloat().IsInt() || v.LessThan(cty.NumberIntVal(0)).True() || v.GreaterThan(cty.NumberIntVal(9)).True() {
				invalid("gzip_level", "Omit the legacy -1 sentinel, or use an explicit level from 0 through 9.")
			}
		}
		// Both attrs may be set to null; any other unknown combination needs
		// manual review because the target rejects simultaneous settings.
		if attrs["gzip_level"] != nil && attrs["compression_codec"] != nil {
			gzip, gzipKnown := known("gzip_level")
			codec, codecKnown := known("compression_codec")
			if (!gzipKnown || !gzip.IsNull()) && (!codecKnown || !codec.IsNull()) {
				invalid("gzip_level", "Configure only one of gzip_level or compression_codec before migrating.")
			}
		}
	case "logging_kafka":
		oneOf("compression_codec", "gzip", "snappy", "lz4")
		oneOf("required_acks", "1", "0", "-1")
		oneOf("auth_method", "plain", "scram-sha-256", "scram-sha-512")
	case "logging_elasticsearch":
		// The SDKv2 schema did not enforce HTTPS. Only inspect a literal
		// string, not a variable, file or other credential-bearing expression.
		if attr := attrs["url"]; attr != nil {
			if literal, ok := attr.Expr.(*hclsyntax.TemplateExpr); ok && literal.IsStringLiteral() {
				v, _ := literal.Value(nil)
				parsed, err := url.Parse(v.AsString())
				if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
					invalid("url", "Configure an HTTPS Elasticsearch endpoint supported by the target provider.")
				}
			}
		}
	}
}
