package alert

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

func FlattenToModel(ctx context.Context, ad *fastly.AlertDefinition) (Model, diag.Diagnostics) {
	var diags diag.Diagnostics

	m := Model{
		ID:     types.StringValue(ad.ID),
		Name:   types.StringValue(ad.Name),
		Source: types.StringValue(ad.Source),
		Metric: types.StringValue(ad.Metric),
	}

	if ad.ServiceID != "" {
		m.ServiceID = types.StringValue(ad.ServiceID)
	} else {
		m.ServiceID = types.StringNull()
	}

	description := strings.TrimSpace(strings.TrimSuffix(ad.Description, managedByTerraform))
	m.Description = types.StringValue(description)

	domains, present := ad.Dimensions["domains"]
	domainsSet, d := flattenDimensionSet(ctx, domains, present)
	diags.Append(d...)

	origins, present := ad.Dimensions["origins"]
	originsSet, d := flattenDimensionSet(ctx, origins, present)
	diags.Append(d...)

	if len(ad.Dimensions) > 0 {
		m.Dimensions = []DimensionsModel{{Domains: domainsSet, Origins: originsSet}}
	}

	if len(ad.EvaluationStrategy) > 0 {
		m.EvaluationStrategy = []EvaluationStrategyModel{flattenEvaluationStrategy(ad.EvaluationStrategy)}
	}

	integrationIDs := ad.IntegrationIDs
	if integrationIDs == nil {
		integrationIDs = []string{}
	}
	ids, d := types.SetValueFrom(ctx, types.StringType, integrationIDs)
	diags.Append(d...)
	m.IntegrationIDs = ids

	return m, diags
}

func flattenDimensionSet(ctx context.Context, values []string, present bool) (types.Set, diag.Diagnostics) {
	if !present {
		return types.SetNull(types.StringType), nil
	}
	return types.SetValueFrom(ctx, types.StringType, values)
}

func flattenEvaluationStrategy(m map[string]any) EvaluationStrategyModel {
	s := EvaluationStrategyModel{
		IgnoreBelow: types.Float64Null(),
	}

	if v, ok := m["type"].(string); ok {
		s.Type = types.StringValue(v)
	}
	if v, ok := m["period"].(string); ok {
		s.Period = types.StringValue(v)
	}
	if v, ok := m["threshold"].(float64); ok {
		s.Threshold = types.Float64Value(v)
	}
	if v, ok := m["ignore_below"].(float64); ok {
		s.IgnoreBelow = types.Float64Value(v)
	}

	return s
}
