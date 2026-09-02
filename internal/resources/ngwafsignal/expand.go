package ngwafsignal

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

func BuildCreateInput(ctx context.Context, plan Model) (*signals.CreateInput, diag.Diagnostics) {
	appliesTo, diags := ngwafrule.ExpandAppliesTo(ctx, plan.AppliesTo)
	if diags.HasError() {
		return nil, diags
	}

	name := service.StringValue(plan.Name)
	description := service.StringValue(plan.Description)

	return &signals.CreateInput{
		Name:        &name,
		Description: &description,
		Scope:       ngwafrule.AccountScope(appliesTo),
	}, diags
}

func BuildUpdateInput(ctx context.Context, signalID string, plan Model) (*signals.UpdateInput, diag.Diagnostics) {
	appliesTo, diags := ngwafrule.ExpandAppliesTo(ctx, plan.AppliesTo)
	if diags.HasError() {
		return nil, diags
	}

	description := service.StringValue(plan.Description)

	return &signals.UpdateInput{
		SignalID:    &signalID,
		Description: &description,
		Scope:       ngwafrule.AccountScope(appliesTo),
	}, diags
}

// BuildGetInput addresses an account signal by ID alone. Read repopulates
// applies_to from the API response, which also makes bare-ID import work.
func BuildGetInput(signalID string) *signals.GetInput {
	return &signals.GetInput{
		SignalID: &signalID,
		Scope:    ngwafrule.AccountScopeByID(),
	}
}

// BuildDeleteInput addresses an account signal by ID alone, matching
// DELETE /ngwaf/v1/signals/{signal_id}.
func BuildDeleteInput(signalID string) *signals.DeleteInput {
	return &signals.DeleteInput{
		SignalID: &signalID,
		Scope:    ngwafrule.AccountScopeByID(),
	}
}
