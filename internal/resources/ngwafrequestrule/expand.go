package ngwafrequestrule

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func BuildCreateInput(ctx context.Context, plan Model) (*rules.CreateInput, diag.Diagnostics) {
	appliesTo, diags := ngwafrule.ExpandAppliesTo(ctx, plan.AppliesTo)
	if diags.HasError() {
		return nil, diags
	}

	input := ngwafrule.NewCreateInput(RuleType, ngwafrule.AccountScope(appliesTo), plan.CommonModel)
	input.Description = new(service.StringValue(plan.Description))
	input.RequestLogging = plan.RequestLogging.ValueStringPointer()
	input.Actions = ngwafrule.ExpandCreateAccountActions(plan.Action)

	return input, diags
}

func BuildUpdateInput(ctx context.Context, ruleID string, plan Model) (*rules.UpdateInput, diag.Diagnostics) {
	appliesTo, diags := ngwafrule.ExpandAppliesTo(ctx, plan.AppliesTo)
	if diags.HasError() {
		return nil, diags
	}

	input := ngwafrule.NewUpdateInput(RuleType, ruleID, ngwafrule.AccountScope(appliesTo), plan.CommonModel)
	input.Description = new(service.StringValue(plan.Description))
	input.RequestLogging = plan.RequestLogging.ValueStringPointer()
	input.Actions = ngwafrule.ExpandUpdateAccountActions(plan.Action)

	return input, diags
}
