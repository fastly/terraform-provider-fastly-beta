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

// analyzeStorageLoggingBlock rejects unverified legacy or target-only shapes
// before any edits are written. Do not evaluate or include credential values
// in findings, even when they are statically known.
func analyzeStorageLoggingBlock(state *analysisState, block *hclsyntax.Block, vcl bool) {
	reject := func(rng hcl.Range, name, remediation string) {
		state.result.Findings = append(state.result.Findings, Finding{
			Severity: SeverityError, Range: rng,
			Message:     fmt.Sprintf("Legacy %s option %q cannot be safely migrated.", block.Type, name),
			Remediation: remediation,
		})
	}
	attrs := block.Body.Attributes
	for name, attr := range attrs {
		if !compatibility.StorageLoggingLegacyField(block.Type, name, vcl) {
			reject(attr.NameRange, name, "Use the published SDKv2 flat endpoint schema; target-only objects and attributes cannot be migrated.")
		}
	}
	required := map[string][]string{
		"logging_s3":           {"name", "bucket_name"},
		"logging_gcs":          {"name", "bucket_name"},
		"logging_bigquery":     {"name", "project_id", "dataset", "table"},
		"logging_blobstorage":  {"name", "container"},
		"logging_cloudfiles":   {"name", "bucket_name", "user", "access_key"},
		"logging_digitalocean": {"name", "bucket_name", "access_key", "secret_key"},
		"logging_openstack":    {"name", "bucket_name", "url", "user", "access_key"},
		"logging_googlepubsub": {"name", "project_id", "topic"},
		"logging_kinesis":      {"name", "topic"},
	}
	for _, name := range required[block.Type] {
		if attrs[name] == nil {
			reject(block.TypeRange, name, "Configure the required legacy endpoint field before migration.")
		}
	}
	known := func(name string) (cty.Value, bool) {
		attr := attrs[name]
		if attr == nil {
			return cty.NilVal, false
		}
		value, diags := attr.Expr.Value(nil)
		return value, !diags.HasErrors() && value.IsKnown()
	}
	for _, name := range required[block.Type] {
		if value, ok := known(name); ok && value.IsNull() {
			reject(attrs[name].NameRange, name,
				"Required legacy endpoint fields cannot be null.")
		}
	}
	if vcl {
		if value, ok := known("placement"); ok && !value.IsNull() &&
			(value.Type() != cty.String || value.AsString() != "none") {
			reject(attrs["placement"].NameRange, "placement", "Known placement values must be \"none\"; unresolved expressions are preserved for target-provider validation.")
		}
		if value, ok := known("format"); ok && !value.IsNull() &&
			(value.Type() != cty.String || strings.TrimSpace(value.AsString()) == "") {
			reject(attrs["format"].NameRange, "format", "Configure a non-empty format or omit it to preserve a verified legacy default.")
		}
		// The legacy BigQuery SDKv2 format default has not been established
		// from historical provider constants; never substitute the Framework
		// default blindly. An explicit non-null format is safe to retain.
		if block.Type == "logging_bigquery" {
			value, ok := known("format")
			if attrs["format"] == nil || (ok && value.IsNull()) {
				reject(block.TypeRange, "format", "Set an explicit legacy BigQuery log format before migration; the historical default has not been verified.")
			}
		}
	}
	// Validate mutually exclusive target authentication modes without
	// inspecting credential contents. Unknown expressions count as configured
	// because we cannot prove that they resolve to null.
	present := func(name string) bool {
		if attrs[name] == nil {
			return false
		}
		value, ok := known(name)
		return !ok || !value.IsNull()
	}
	// When credentials are explicitly configured, do not depend on
	// unspecified environment defaults to fill in a missing partner field.
	// Omitted credential blocks can still use the provider's existing
	// environment fallback where supported.
	pairs := map[string][2]string{
		"logging_s3":           {"s3_access_key", "s3_secret_key"},
		"logging_kinesis":      {"access_key", "secret_key"},
		"logging_gcs":          {"user", "secret_key"},
		"logging_bigquery":     {"email", "secret_key"},
		"logging_googlepubsub": {"user", "secret_key"},
		"logging_blobstorage":  {"account_name", "sas_token"},
	}
	if fields, ok := pairs[block.Type]; ok && present(fields[0]) != present(fields[1]) {
		name := fields[0]
		if !present(name) {
			name = fields[1]
		}
		reject(attrs[name].NameRange, name,
			"Configure both credential fields or omit both; a partial credential pair cannot be verified.")
	}
	switch block.Type {
	case "logging_s3":
		if present("s3_iam_role") && (present("s3_access_key") || present("s3_secret_key")) {
			reject(attrs["s3_iam_role"].NameRange, "s3_iam_role",
				"Choose either an S3 IAM role or access/secret keys before migration.")
		}
	case "logging_kinesis":
		if present("iam_role") && (present("access_key") || present("secret_key")) {
			reject(attrs["iam_role"].NameRange, "iam_role",
				"Choose either a Kinesis IAM role or access/secret keys before migration.")
		}
	case "logging_gcs", "logging_googlepubsub":
		if present("account_name") && (present("user") || present("secret_key")) {
			reject(attrs["account_name"].NameRange, "account_name",
				"Choose either the account name or the email and secret key before migration.")
		}
	case "logging_bigquery":
		if present("account_name") && (present("email") || present("secret_key")) {
			reject(attrs["account_name"].NameRange, "account_name",
				"Choose either the account name or the email and secret key before migration.")
		}
	}
	if attrs["gzip_level"] != nil && attrs["compression_codec"] != nil {
		gzip, gok := known("gzip_level")
		codec, cok := known("compression_codec")
		if (!gok || !gzip.IsNull()) && (!cok || !codec.IsNull()) {
			reject(attrs["gzip_level"].NameRange, "gzip_level",
				"Configure only one of gzip_level or compression_codec before migration.")
		}
	}
	if value, ok := known("gzip_level"); ok && !value.IsNull() {
		if value.Type() != cty.Number || !value.AsBigFloat().IsInt() ||
			value.LessThan(cty.NumberIntVal(0)).True() || value.GreaterThan(cty.NumberIntVal(9)).True() {
			reject(attrs["gzip_level"].NameRange, "gzip_level", "Use an explicit gzip level from 0 through 9, or omit the legacy -1 sentinel.")
		}
	}
	if value, ok := known("compression_codec"); ok && !value.IsNull() {
		if value.Type() != cty.String || !slices.Contains([]string{"gzip", "snappy", "zstd"}, value.AsString()) {
			reject(attrs["compression_codec"].NameRange, "compression_codec", "Use a compression codec supported by the target endpoint.")
		}
	}
}
