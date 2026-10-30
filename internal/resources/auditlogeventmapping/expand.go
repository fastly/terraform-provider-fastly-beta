package auditlogeventmapping

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v18/fastly/notifications/v1/eventmappings"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func expandStringSet(ctx context.Context, s types.Set, diags *diag.Diagnostics) []string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}

	var out []string
	diags.Append(s.ElementsAs(ctx, &out, false)...)
	return out
}

func BuildCreateInput(ctx context.Context, plan Model) (*eventmappings.CreateInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	input := &eventmappings.CreateInput{
		Name:           new(service.StringValue(plan.Name)),
		ScopeType:      new(service.StringValue(plan.ScopeType)),
		ScopeIDs:       expandStringSet(ctx, plan.ScopeIDs, &diags),
		EventTypes:     expandStringSet(ctx, plan.EventTypes, &diags),
		IntegrationIDs: expandStringSet(ctx, plan.IntegrationIDs, &diags),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = new(plan.Description.ValueString())
	}

	return input, diags
}

func BuildUpdateInput(ctx context.Context, id string, plan Model) (*eventmappings.UpdateInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	input := &eventmappings.UpdateInput{
		MappingID:      new(id),
		Name:           new(service.StringValue(plan.Name)),
		ScopeType:      new(service.StringValue(plan.ScopeType)),
		ScopeIDs:       expandStringSet(ctx, plan.ScopeIDs, &diags),
		EventTypes:     expandStringSet(ctx, plan.EventTypes, &diags),
		IntegrationIDs: expandStringSet(ctx, plan.IntegrationIDs, &diags),
	}
	// Always send description so removing it from config clears it server-side.
	input.Description = new(plan.Description.ValueString())

	return input, diags
}
