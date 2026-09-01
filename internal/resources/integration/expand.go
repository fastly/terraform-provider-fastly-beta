package integration

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func expandConfig(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}

	var result map[string]string
	diags.Append(m.ElementsAs(ctx, &result, false)...)
	return result
}

func BuildCreateInput(ctx context.Context, plan Model) (*fastly.CreateIntegrationInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	input := &fastly.CreateIntegrationInput{
		Config: expandConfig(ctx, plan.Config, &diags),
		Name:   new(service.StringValue(plan.Name)),
		Type:   new(service.StringValue(plan.Type)),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = new(plan.Description.ValueString())
	}

	return input, diags
}

func BuildUpdateInput(ctx context.Context, id string, plan Model) (*fastly.UpdateIntegrationInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	input := &fastly.UpdateIntegrationInput{
		ID:     id,
		Config: expandConfig(ctx, plan.Config, &diags),
		Name:   new(service.StringValue(plan.Name)),
		Type:   new(service.StringValue(plan.Type)),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = new(plan.Description.ValueString())
	}

	return input, diags
}
