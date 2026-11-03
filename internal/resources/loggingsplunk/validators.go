package loggingsplunk

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// authenticationRequired enforces that a token is set, directly or via
// FASTLY_SPLUNK_TOKEN, matching the live provider's Required+EnvDefaultFunc
// behavior — the schema Default alone only supplies a fallback value, not a
// requirement.
type authenticationRequired struct{}

func (authenticationRequired) Description(_ context.Context) string {
	return "authentication must set token, directly or via FASTLY_SPLUNK_TOKEN"
}

func (v authenticationRequired) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (authenticationRequired) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsUnknown() {
		return
	}

	if effectiveAuthToken(ctx, req.ConfigValue) != "" {
		return
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Missing Splunk authentication credentials",
		"`authentication` must set `token` — directly or via the FASTLY_SPLUNK_TOKEN environment variable.",
	)
}

// effectiveAuthToken returns the configured token, falling back to
// FASTLY_SPLUNK_TOKEN when omitted (an explicit "" is not rescued by the
// env var).
func effectiveAuthToken(ctx context.Context, obj types.Object) string {
	if !obj.IsNull() && !obj.IsUnknown() {
		if v, ok := obj.Attributes()["token"]; ok {
			if sv, ok := v.(types.String); ok && !sv.IsNull() && !sv.IsUnknown() {
				return sv.ValueString()
			}
		}
	}
	return envStringDefault(ctx, splunkTokenEnvVar).ValueString()
}
