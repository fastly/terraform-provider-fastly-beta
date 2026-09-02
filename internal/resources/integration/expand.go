package integration

import (
	"context"
	"fmt"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func expandMap(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}

	var result map[string]string
	diags.Append(m.ElementsAs(ctx, &result, false)...)
	return result
}

// expandConfig merges the non-sensitive `config` map with the sensitive
// `authentication` map into the single map[string]string the Fastly API expects.
func expandConfig(ctx context.Context, plan Model, diags *diag.Diagnostics) map[string]string {
	config := expandMap(ctx, plan.Config, diags)
	auth := expandMap(ctx, plan.Authentication, diags)
	if len(config) == 0 && len(auth) == 0 {
		return nil
	}

	merged := make(map[string]string, len(config)+len(auth))
	maps.Copy(merged, config)
	for k, v := range auth {
		if _, exists := merged[k]; exists {
			diags.AddError("Duplicate integration config key",
				fmt.Sprintf("%q is set in both `config` and `authentication`; each key may only be set once.", k))
			continue
		}
		merged[k] = v
	}

	return merged
}

func BuildCreateInput(ctx context.Context, plan Model) (*fastly.CreateIntegrationInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	input := &fastly.CreateIntegrationInput{
		Config: expandConfig(ctx, plan, &diags),
		Name:   new(service.StringValue(plan.Name)),
		Type:   new(service.StringValue(plan.Type)),
	}
	if !plan.Description.IsUnknown() {
		input.Description = new(plan.Description.ValueString())
	}

	return input, diags
}

func BuildUpdateInput(ctx context.Context, id string, plan Model) (*fastly.UpdateIntegrationInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	input := &fastly.UpdateIntegrationInput{
		ID:     id,
		Config: expandConfig(ctx, plan, &diags),
		Name:   new(service.StringValue(plan.Name)),
		Type:   new(service.StringValue(plan.Type)),
	}
	// plan.Description.ValueString() returns "" for a null value, so removing
	// description from config clears it server-side rather than leaving it untouched.
	if !plan.Description.IsUnknown() {
		input.Description = new(plan.Description.ValueString())
	}

	return input, diags
}
