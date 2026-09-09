package validation

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// NotBlank rejects an explicit "" on an Optional+Computed attribute whose
// Default is non-empty. A Default only fires for an omitted (null) value, but
// the Fastly API substitutes its own default for an omitted or "" value
// alike - so an explicit "" can never round-trip, and apply fails with
// "Provider produced inconsistent result after apply". Reject it at validate
// time instead. attrName names the attribute in the error message.
func NotBlank(attrName string) validator.String {
	return notBlank{attrName: attrName}
}

type notBlank struct {
	attrName string
}

func (v notBlank) Description(_ context.Context) string {
	return fmt.Sprintf("%s must not be explicitly set to an empty string; omit the attribute to use its default", v.attrName)
}

func (v notBlank) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v notBlank) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if req.ConfigValue.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			fmt.Sprintf("Invalid `%s`", v.attrName),
			fmt.Sprintf("`%s` cannot be explicitly set to an empty string. The Fastly API always substitutes its own default in that case, which can never match an explicitly configured \"\". Remove the `%s` attribute entirely to use the default.", v.attrName, v.attrName),
		)
	}
}
