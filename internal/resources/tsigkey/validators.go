package tsigkey

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// base64Validator catches a malformed secret at plan time instead of apply.
type base64Validator struct{}

func (base64Validator) Description(_ context.Context) string {
	return "value must be valid Base64"
}

func (v base64Validator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (base64Validator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	v := req.ConfigValue.ValueString()
	if _, err := base64.StdEncoding.DecodeString(v); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Attribute Value",
			fmt.Sprintf("must be valid Base64, got: %q", v),
		)
	}
}
