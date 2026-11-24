package logginggooglepubsub

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// notTrimmed rejects a string with leading or trailing whitespace (e.g.
// \n\t\r\f). The Fastly API silently mishandles a Pub/Sub secret_key with
// such whitespace (per loggingbigquery's notTrimmed) — Pub/Sub's secret_key
// is the same PEM private-key material, so the same check is applied here.
type notTrimmed struct{}

func (notTrimmed) Description(_ context.Context) string {
	return "value must not contain leading or trailing whitespace"
}

func (v notTrimmed) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (notTrimmed) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	v := req.ConfigValue.ValueString()
	if v != strings.TrimSpace(v) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Value",
			"must not contain leading or trailing whitespace characters (e.g., \\n\\t\\r\\f). Consider trimming the value.",
		)
	}
}

// authenticationEitherOr enforces that the authentication block resolves to
// either account_name, or both email and secret_key. The Fastly API accepts
// account_name as an alternative to the email/secret_key pair, but rejects a
// request providing neither or only one of email/secret_key, so this is
// caught at plan/validate time instead. Each field's effective value falls
// back to its environment variable default (mirroring the schema Default
// handlers) so a config that legitimately relies on the environment for one
// or more fields is not flagged.
type authenticationEitherOr struct{}

func (authenticationEitherOr) Description(_ context.Context) string {
	return "authentication must set account_name, or both email and secret_key"
}

func (v authenticationEitherOr) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (authenticationEitherOr) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsUnknown() {
		return
	}

	if hasEffectiveAccountName(ctx, req.ConfigValue) {
		return
	}

	email := hasEffectiveAuthValue(ctx, req.ConfigValue, "email", "FASTLY_GOOGLE_PUBSUB_EMAIL")
	secretKey := hasEffectiveAuthValue(ctx, req.ConfigValue, "secret_key", "FASTLY_GOOGLE_PUBSUB_SECRET_KEY")
	if email && secretKey {
		return
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Missing Pub/Sub authentication credentials",
		"`authentication` must set `account_name`, or both `email` and `secret_key` — directly or via the "+
			"FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME (or FASTLY_GCS_ACCOUNT_NAME), FASTLY_GOOGLE_PUBSUB_EMAIL, and FASTLY_GOOGLE_PUBSUB_SECRET_KEY environment variables.",
	)
}

// hasEffectiveAuthValue reports whether the raw configured credential is
// potentially usable, or an omitted field has a nonempty environment default.
// Unknown expressions are provisionally accepted during terraform validate;
// they must be checked once known during plan/apply. An explicitly empty
// string is invalid and must not fall back to an environment variable.
func hasEffectiveAuthValue(ctx context.Context, obj types.Object, attrName, envVar string) bool {
	if !obj.IsNull() && !obj.IsUnknown() {
		if v, ok := obj.Attributes()[attrName]; ok {
			if sv, ok := v.(types.String); ok && !sv.IsNull() {
				return sv.IsUnknown() || sv.ValueString() != ""
			}
		}
	}
	return envStringDefault(ctx, envVar).ValueString() != ""
}

// hasEffectiveAccountName checks configured account_name before using its
// environment fallback. A configured unknown value is provisionally accepted;
// a configured empty string must not silently use an environment variable.
func hasEffectiveAccountName(ctx context.Context, obj types.Object) bool {
	if !obj.IsNull() && !obj.IsUnknown() {
		if v, ok := obj.Attributes()["account_name"]; ok {
			if sv, ok := v.(types.String); ok && !sv.IsNull() {
				return sv.IsUnknown() || sv.ValueString() != ""
			}
		}
	}
	return accountNameEnvValue(ctx).ValueString() != ""
}
