package tlssubscription

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// lowercaseString rejects uppercase letters. The Fastly API silently lowercases domains and
// common names, which would otherwise cause a permanent plan/state mismatch.
type lowercaseString struct{}

func (lowercaseString) Description(_ context.Context) string {
	return "must not contain uppercase letters"
}

func (v lowercaseString) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (lowercaseString) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v := req.ConfigValue.ValueString()
	if v != strings.ToLower(v) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid common_name", fmt.Sprintf("tls subscription 'common_name' must not contain uppercase letters: %s", v))
	}
}

// lowercaseSetElements is the domains-set equivalent of lowercaseString.
type lowercaseSetElements struct{}

func (lowercaseSetElements) Description(_ context.Context) string {
	return "elements must not contain uppercase letters"
}

func (v lowercaseSetElements) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (lowercaseSetElements) ValidateSet(_ context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	for _, elem := range req.ConfigValue.Elements() {
		s, ok := elem.(types.String)
		if !ok || s.IsNull() || s.IsUnknown() {
			continue
		}
		v := s.ValueString()
		if v != strings.ToLower(v) {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid domains", fmt.Sprintf("tls subscription 'domains' must not contain uppercase letters: %s", v))
		}
	}
}
