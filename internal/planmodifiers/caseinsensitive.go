// Package planmodifiers holds plan modifiers shared across resource packages.
package planmodifiers

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// CaseInsensitiveState preserves the prior state value when the configured value is
// case-insensitively equal to it. Pair this with an enum attribute whose value is lowercased
// before being sent to the API and read back from state in that lowercase form — without it, a
// differently-cased config value (e.g. "PASS") would never converge with state and Terraform
// would show a persistent plan diff on every run.
func CaseInsensitiveState() planmodifier.String {
	return caseInsensitiveState{}
}

type caseInsensitiveState struct{}

func (m caseInsensitiveState) Description(_ context.Context) string {
	return "Preserves the prior state value when the configured value differs only in case."
}

func (m caseInsensitiveState) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m caseInsensitiveState) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if strings.EqualFold(req.StateValue.ValueString(), req.ConfigValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}
