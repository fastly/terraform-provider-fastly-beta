package alert

import (
	"context"
	"errors"

	"github.com/fastly/terraform-provider-fastly/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

func descriptionWithManagedSuffix(description string) string {
	if description == "" {
		return managedByTerraform
	}
	return description + " " + managedByTerraform
}

func validateSourceWithServiceID(source, serviceID string) error {
	if source != "" && source != "stats" && serviceID == "" {
		return errors.New(badAlertSourceServiceIDConfig)
	}
	return nil
}

func setToStringSlice(ctx context.Context, s types.Set, diags *diag.Diagnostics) []string {
	if s.IsNull() || s.IsUnknown() {
		return []string{}
	}

	values := make([]string, 0, len(s.Elements()))
	diags.Append(s.ElementsAs(ctx, &values, false)...)
	return values
}

// Only sends keys that were actually configured; omitting one has the same effect on the API as an empty list.
func dimensionsMap(ctx context.Context, dimensions []DimensionsModel, diags *diag.Diagnostics) map[string][]string {
	data := map[string][]string{}
	if len(dimensions) != 1 {
		return data
	}

	d := dimensions[0]
	if !d.Domains.IsNull() {
		data["domains"] = setToStringSlice(ctx, d.Domains, diags)
	}
	if !d.Origins.IsNull() {
		data["origins"] = setToStringSlice(ctx, d.Origins, diags)
	}
	return data
}

func evaluationStrategyMap(strategy []EvaluationStrategyModel) map[string]any {
	if len(strategy) != 1 {
		return nil
	}

	s := strategy[0]
	m := map[string]any{
		"type":      s.Type.ValueString(),
		"period":    s.Period.ValueString(),
		"threshold": s.Threshold.ValueFloat64(),
	}

	if !s.IgnoreBelow.IsNull() && !s.IgnoreBelow.IsUnknown() {
		m["ignore_below"] = s.IgnoreBelow.ValueFloat64()
	}

	return m
}

// commonFields holds the attributes shared by create and update requests.
type commonFields struct {
	Description        string
	Dimensions         map[string][]string
	EvaluationStrategy map[string]any
	IntegrationIDs     []string
	Metric             string
	Name               string
}

func buildCommonFields(ctx context.Context, plan Model) (commonFields, diag.Diagnostics) {
	var diags diag.Diagnostics

	f := commonFields{
		Description:        descriptionWithManagedSuffix(service.StringValue(plan.Description)),
		Dimensions:         dimensionsMap(ctx, plan.Dimensions, &diags),
		EvaluationStrategy: evaluationStrategyMap(plan.EvaluationStrategy),
		IntegrationIDs:     setToStringSlice(ctx, plan.IntegrationIDs, &diags),
		Metric:             service.StringValue(plan.Metric),
		Name:               service.StringValue(plan.Name),
	}

	return f, diags
}

func BuildCreateInput(ctx context.Context, plan Model) (*fastly.CreateAlertDefinitionInput, diag.Diagnostics) {
	f, diags := buildCommonFields(ctx, plan)

	input := &fastly.CreateAlertDefinitionInput{
		Description:        &f.Description,
		Dimensions:         f.Dimensions,
		EvaluationStrategy: f.EvaluationStrategy,
		IntegrationIDs:     f.IntegrationIDs,
		Metric:             &f.Metric,
		Name:               &f.Name,
		ServiceID:          new(service.StringValue(plan.ServiceID)),
		Source:             new(service.StringValue(plan.Source)),
	}

	return input, diags
}

func BuildUpdateInput(ctx context.Context, id string, plan Model) (*fastly.UpdateAlertDefinitionInput, diag.Diagnostics) {
	f, diags := buildCommonFields(ctx, plan)

	input := &fastly.UpdateAlertDefinitionInput{
		ID:                 &id,
		Description:        &f.Description,
		Dimensions:         f.Dimensions,
		EvaluationStrategy: f.EvaluationStrategy,
		IntegrationIDs:     f.IntegrationIDs,
		Metric:             &f.Metric,
		Name:               &f.Name,
	}

	return input, diags
}
