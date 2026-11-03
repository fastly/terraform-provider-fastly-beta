package loggingelasticsearch

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// httpsURL requires url to be a valid URL with a non-empty host and the
// https:// scheme, matching the Fastly API's documented requirement ("Must
// use HTTPS").
type httpsURL struct{}

func (httpsURL) Description(_ context.Context) string {
	return "must be a valid URL using the https:// scheme"
}

func (v httpsURL) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (httpsURL) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid URL",
			fmt.Sprintf("must be a valid URL using the https:// scheme, got %q.", value),
		)
	}
}

// trimmedValidator rejects a string with leading or trailing whitespace.
// Applied to tls_ca_cert, tls_client_cert, and tls_client_key — unlike most
// other logging endpoints, this one enforces it.
type trimmedValidator struct{}

func (trimmedValidator) Description(_ context.Context) string {
	return "value must not contain leading or trailing whitespace characters (e.g. \\n\\t\\r\\f)"
}

func (v trimmedValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (trimmedValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	v := req.ConfigValue.ValueString()
	if v != strings.TrimSpace(v) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid whitespace",
			req.Path.String()+" must not contain leading or trailing whitespace characters (e.g. \\n\\t\\r\\f). Consider using the trimspace() function.",
		)
	}
}
