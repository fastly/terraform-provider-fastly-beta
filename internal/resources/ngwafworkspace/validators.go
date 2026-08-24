package ngwafworkspace

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// blockingResponseCodeValidator accepts the redirect codes 301 and 302, or
// any generic HTTP status code in the 400-599 range, matching what the
// Fastly API allows for a workspace's default_blocking_response_code.
type blockingResponseCodeValidator struct{}

func (blockingResponseCodeValidator) Description(_ context.Context) string {
	return "value must be 301, 302, or between 400 and 599"
}

func (v blockingResponseCodeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (blockingResponseCodeValidator) ValidateInt64(_ context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	v := req.ConfigValue.ValueInt64()
	if v == 301 || v == 302 || (v >= 400 && v <= 599) {
		return
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid Attribute Value",
		fmt.Sprintf("value must be 301, 302, or between 400 and 599, got: %d", v),
	)
}
