package tlssubscription

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// subscriptionIsMutable reports whether the subscription's current state permits an in-place
// update to domains/common_name/configuration_id; a nil prior state means create, not update,
// so there's nothing to replace.
func subscriptionIsMutable(ctx context.Context, state tfsdk.State) bool {
	if state.Raw.IsNull() {
		return true
	}
	var current types.String
	if diags := state.GetAttribute(ctx, path.Root("state"), &current); diags.HasError() {
		return true
	}
	v := current.ValueString()
	return v == "issued" || v == "pending"
}

// requiresReplaceUnlessMutableString forces replacement of a changed attribute unless the
// subscription is in a state ("issued" or "pending") the API allows updating in place.
type requiresReplaceUnlessMutableString struct{}

func (requiresReplaceUnlessMutableString) Description(_ context.Context) string {
	return "requires replacement unless the subscription is in the `issued` or `pending` state"
}

func (m requiresReplaceUnlessMutableString) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (requiresReplaceUnlessMutableString) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.Equal(req.PlanValue) {
		return
	}
	if !subscriptionIsMutable(ctx, req.State) {
		resp.RequiresReplace = true
	}
}

// requiresReplaceUnlessMutableSet is the domains-set equivalent of requiresReplaceUnlessMutableString.
type requiresReplaceUnlessMutableSet struct{}

func (requiresReplaceUnlessMutableSet) Description(_ context.Context) string {
	return "requires replacement unless the subscription is in the `issued` or `pending` state"
}

func (m requiresReplaceUnlessMutableSet) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (requiresReplaceUnlessMutableSet) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	if req.StateValue.Equal(req.PlanValue) {
		return
	}
	if !subscriptionIsMutable(ctx, req.State) {
		resp.RequiresReplace = true
	}
}
