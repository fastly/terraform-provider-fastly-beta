package alert

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestValidateSourceWithServiceID(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		serviceID string
		wantErr   bool
	}{
		{"stats without service_id is valid", "stats", "", false},
		{"stats with service_id is valid", "stats", "abc123", false},
		{"domains without service_id is invalid", "domains", "", true},
		{"domains with service_id is valid", "domains", "abc123", false},
		{"origins without service_id is invalid", "origins", "", true},
		{"origins with service_id is valid", "origins", "abc123", false},
		{"empty source without service_id is valid", "", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateSourceWithServiceID(c.source, c.serviceID)
			if c.wantErr {
				assert.EqualError(t, err, badAlertSourceServiceIDConfig)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDescriptionWithManagedSuffix(t *testing.T) {
	assert.Equal(t, "Managed by Terraform", descriptionWithManagedSuffix(""))
	assert.Equal(t, "my description Managed by Terraform", descriptionWithManagedSuffix("my description"))
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		Name:           types.StringValue("alert name"),
		Description:    types.StringValue("my description"),
		ServiceID:      types.StringValue("svc123"),
		Source:         types.StringValue("domains"),
		Metric:         types.StringValue("status_5xx"),
		IntegrationIDs: mustSet(t, "int1", "int2"),
		Dimensions: []DimensionsModel{{
			Domains: mustSet(t, "example.com"),
			Origins: types.SetNull(types.StringType),
		}},
		EvaluationStrategy: []EvaluationStrategyModel{{
			Type:      types.StringValue("above_threshold"),
			Period:    types.StringValue("5m"),
			Threshold: types.Float64Value(10),
		}},
	}

	input, diags := BuildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError())

	assert.Equal(t, "my description Managed by Terraform", fastly.ToValue(input.Description))
	assert.Equal(t, "alert name", fastly.ToValue(input.Name))
	assert.Equal(t, "svc123", fastly.ToValue(input.ServiceID))
	assert.Equal(t, "domains", fastly.ToValue(input.Source))
	assert.Equal(t, "status_5xx", fastly.ToValue(input.Metric))
	assert.ElementsMatch(t, []string{"int1", "int2"}, input.IntegrationIDs)

	assert.Equal(t, map[string][]string{"domains": {"example.com"}}, input.Dimensions)

	assert.Equal(t, map[string]any{
		"type":      "above_threshold",
		"period":    "5m",
		"threshold": float64(10),
	}, input.EvaluationStrategy)
}

func TestBuildCreateInput_emptyDescriptionAndUnsetCollections(t *testing.T) {
	plan := Model{
		Name:      types.StringValue("alert name"),
		ServiceID: types.StringNull(),
		Source:    types.StringValue("stats"),
		Metric:    types.StringValue("status_5xx"),
		EvaluationStrategy: []EvaluationStrategyModel{{
			Type:      types.StringValue("above_threshold"),
			Period:    types.StringValue("5m"),
			Threshold: types.Float64Value(10),
		}},
	}

	input, diags := BuildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError())

	assert.Equal(t, "Managed by Terraform", fastly.ToValue(input.Description))
	assert.Equal(t, []string{}, input.IntegrationIDs)
	assert.Equal(t, map[string][]string{}, input.Dimensions)
}

func TestEvaluationStrategyMap_ignoreBelow(t *testing.T) {
	m := evaluationStrategyMap([]EvaluationStrategyModel{{
		Type:        types.StringValue("percent_increase"),
		Period:      types.StringValue("2m"),
		Threshold:   types.Float64Value(0.25),
		IgnoreBelow: types.Float64Value(10),
	}})

	assert.Equal(t, map[string]any{
		"type":         "percent_increase",
		"period":       "2m",
		"threshold":    0.25,
		"ignore_below": float64(10),
	}, m)
}

func TestEvaluationStrategyMap_ignoreBelowZeroOmitted(t *testing.T) {
	m := evaluationStrategyMap([]EvaluationStrategyModel{{
		Type:        types.StringValue("above_threshold"),
		Period:      types.StringValue("5m"),
		Threshold:   types.Float64Value(10),
		IgnoreBelow: types.Float64Value(0),
	}})

	_, ok := m["ignore_below"]
	assert.False(t, ok, "a zero-value ignore_below should not be sent")
}

func TestFlattenToModel_descriptionSuffixStripped(t *testing.T) {
	ad := &fastly.AlertDefinition{
		ID:                 "def123",
		Name:               "my alert",
		ServiceID:          "svc123",
		Source:             "stats",
		Metric:             "status_5xx",
		Description:        "my description Managed by Terraform",
		EvaluationStrategy: map[string]any{"type": "above_threshold", "period": "5m", "threshold": float64(10)},
	}

	m, diags := FlattenToModel(context.Background(), ad)
	require.False(t, diags.HasError())
	assert.Equal(t, types.StringValue("my description"), m.Description)
}

func TestFlattenToModel_descriptionOnlyManagedSuffixLeftUnset(t *testing.T) {
	ad := &fastly.AlertDefinition{
		ID:                 "def123",
		Name:               "my alert",
		Source:             "stats",
		Metric:             "status_5xx",
		Description:        "Managed by Terraform",
		EvaluationStrategy: map[string]any{"type": "above_threshold", "period": "5m", "threshold": float64(10)},
	}

	m, diags := FlattenToModel(context.Background(), ad)
	require.False(t, diags.HasError())
	assert.Equal(t, types.StringNull(), m.Description)
}

func TestFlattenToModel_dimensionsOnlyConfiguredKeyPresent(t *testing.T) {
	ad := &fastly.AlertDefinition{
		ID:         "def123",
		Name:       "my alert",
		Source:     "domains",
		ServiceID:  "svc123",
		Metric:     "status_5xx",
		Dimensions: map[string][]string{"domains": {"example.com", "fastly.com"}},
	}

	m, diags := FlattenToModel(context.Background(), ad)
	require.False(t, diags.HasError())
	require.Len(t, m.Dimensions, 1)

	var domains []string
	diags = m.Dimensions[0].Domains.ElementsAs(context.Background(), &domains, false)
	require.False(t, diags.HasError())
	assert.ElementsMatch(t, []string{"example.com", "fastly.com"}, domains)

	assert.True(t, m.Dimensions[0].Origins.IsNull())
}

func TestFlattenToModel_noDimensionsMeansNoBlock(t *testing.T) {
	ad := &fastly.AlertDefinition{
		ID:     "def123",
		Name:   "my alert",
		Source: "stats",
		Metric: "status_5xx",
	}

	m, diags := FlattenToModel(context.Background(), ad)
	require.False(t, diags.HasError())
	assert.Nil(t, m.Dimensions)
}

func TestFlattenToModel_emptyServiceIDStaysNull(t *testing.T) {
	ad := &fastly.AlertDefinition{
		ID:        "def123",
		Name:      "my alert",
		Source:    "stats",
		Metric:    "status_5xx",
		ServiceID: "",
	}

	m, diags := FlattenToModel(context.Background(), ad)
	require.False(t, diags.HasError())
	assert.Equal(t, types.StringNull(), m.ServiceID)
}

func TestFlattenToModel_integrationIDs(t *testing.T) {
	ad := &fastly.AlertDefinition{
		ID:             "def123",
		Name:           "my alert",
		Source:         "stats",
		Metric:         "status_5xx",
		IntegrationIDs: []string{"int1", "int2"},
	}

	m, diags := FlattenToModel(context.Background(), ad)
	require.False(t, diags.HasError())

	var ids []string
	diags = m.IntegrationIDs.ElementsAs(context.Background(), &ids, false)
	require.False(t, diags.HasError())
	assert.ElementsMatch(t, []string{"int1", "int2"}, ids)
}

func TestFlattenToModel_noIntegrationIDsIsNull(t *testing.T) {
	ad := &fastly.AlertDefinition{
		ID:     "def123",
		Name:   "my alert",
		Source: "stats",
		Metric: "status_5xx",
	}

	m, diags := FlattenToModel(context.Background(), ad)
	require.False(t, diags.HasError())
	assert.Equal(t, types.SetNull(types.StringType), m.IntegrationIDs)
}

func TestFlattenEvaluationStrategy_ignoreBelowPresentAndAbsent(t *testing.T) {
	withIgnoreBelow := flattenEvaluationStrategy(map[string]any{
		"type": "percent_increase", "period": "2m", "threshold": float64(0.25), "ignore_below": float64(10),
	})
	assert.Equal(t, types.Float64Value(10), withIgnoreBelow.IgnoreBelow)

	withoutIgnoreBelow := flattenEvaluationStrategy(map[string]any{
		"type": "above_threshold", "period": "5m", "threshold": float64(10),
	})
	assert.Equal(t, types.Float64Null(), withoutIgnoreBelow.IgnoreBelow)
}

func mustSet(t *testing.T, values ...string) types.Set {
	t.Helper()
	elements := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elements = append(elements, types.StringValue(v))
	}
	s, diags := types.SetValue(types.StringType, elements)
	require.False(t, diags.HasError())
	return s
}
