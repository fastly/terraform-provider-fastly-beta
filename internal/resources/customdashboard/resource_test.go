package customdashboard

import (
	"context"
	"testing"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func testDashboardModel() Model {
	metrics, _ := types.ListValueFrom(context.Background(), types.StringType, []string{"requests"})
	return Model{
		Name:        types.StringValue("dashboard"),
		Description: types.StringValue("description"),
		DashboardItem: []DashboardItemModel{
			{
				Key:      types.StringValue("requests"),
				ID:       types.StringNull(),
				Title:    types.StringValue("Requests"),
				Subtitle: types.StringValue("Edge requests"),
				Span:     types.Int64Value(4),
				DataSource: []DataSourceModel{{
					Type: types.StringValue("stats.edge"),
					Config: []DataSourceConfigModel{{
						Metrics: metrics,
					}},
				}},
				Visualization: []VisualizationModel{{
					Config: []VisualizationConfigModel{{
						PlotType:          types.StringValue("bar"),
						CalculationMethod: types.StringValue("sum"),
						Format:            types.StringValue("number"),
					}},
				}},
			},
		},
	}
}

func TestExpandItemsOmitsFastlyIDForNewItem(t *testing.T) {
	model := testDashboardModel()
	got, diags := expandItems(context.Background(), model.DashboardItem)
	require.False(t, diags.HasError(), diags)
	require.Len(t, got, 1)

	require.Empty(t, got[0].ID)
	require.Equal(t, "Requests", got[0].Title)
	require.Equal(t, []string{"requests"}, got[0].DataSource.Config.Metrics)
	require.Equal(t, fastly.VisualizationTypeChart, got[0].Visualization.Type)
	require.Equal(t, fastly.PlotType("bar"), got[0].Visualization.Config.PlotType)
	require.NotNil(t, got[0].Visualization.Config.CalculationMethod)
	require.Equal(t, fastly.CalculationMethod("sum"), *got[0].Visualization.Config.CalculationMethod)
	require.NotNil(t, got[0].Visualization.Config.Format)
	require.Equal(t, fastly.VisualizationFormat("number"), *got[0].Visualization.Config.Format)
}

func TestBindIDsByKeySurvivesReorderingAndLeavesNewItemsUnbound(t *testing.T) {
	previous := []DashboardItemModel{
		{Key: types.StringValue("requests"), ID: types.StringValue("api-a")},
		{Key: types.StringValue("errors"), ID: types.StringValue("api-b")},
	}
	desired := []DashboardItemModel{
		{Key: types.StringValue("errors"), ID: types.StringUnknown()},
		{Key: types.StringValue("new-item"), ID: types.StringUnknown()},
		{Key: types.StringValue("requests"), ID: types.StringUnknown()},
	}

	bindIDsByKey(desired, previous)

	require.Equal(t, "api-b", desired[0].ID.ValueString())
	require.True(t, desired[1].ID.IsNull())
	require.Equal(t, "api-a", desired[2].ID.ValueString())
}

func TestBindIDsByKeyAdoptsSyntheticImportedIdentityByContent(t *testing.T) {
	previousModel := testDashboardModel()
	previous := previousModel.DashboardItem
	previous[0].Key = types.StringValue("api-a")
	previous[0].ID = types.StringValue("api-a")

	desiredModel := testDashboardModel()
	desired := desiredModel.DashboardItem
	desired[0].ID = types.StringUnknown()

	bindIDsByKey(desired, previous)

	require.Equal(t, "requests", desired[0].Key.ValueString())
	require.Equal(t, "api-a", desired[0].ID.ValueString())
}

func TestBindIDsByKeyDoesNotAdoptChangedConfiguredKey(t *testing.T) {
	previousModel := testDashboardModel()
	previous := previousModel.DashboardItem
	previous[0].Key = types.StringValue("old-key")
	previous[0].ID = types.StringValue("api-a")

	desiredModel := testDashboardModel()
	desired := desiredModel.DashboardItem
	desired[0].Key = types.StringValue("new-key")
	desired[0].ID = types.StringUnknown()

	bindIDsByKey(desired, previous)

	require.True(t, desired[0].ID.IsNull())
}

func TestBindIDsByKeyDoesNotGuessAmbiguousImportedIdentity(t *testing.T) {
	previousModel := testDashboardModel()
	first := previousModel.DashboardItem[0]
	first.Key = types.StringValue("api-a")
	first.ID = types.StringValue("api-a")
	second := first
	second.Key = types.StringValue("api-b")
	second.ID = types.StringValue("api-b")

	desiredModel := testDashboardModel()
	desired := desiredModel.DashboardItem
	desired[0].ID = types.StringUnknown()

	bindIDsByKey(desired, []DashboardItemModel{first, second})

	require.True(t, desired[0].ID.IsNull())
}

func TestFlattenDashboardPreservesTerraformKeysByFastlyID(t *testing.T) {
	state := Model{
		DashboardItem: []DashboardItemModel{
			{Key: types.StringValue("requests"), ID: types.StringValue("api-a")},
			{Key: types.StringValue("errors"), ID: types.StringValue("api-b")},
		},
	}
	remote := &fastly.ObservabilityCustomDashboard{
		ID:   "dashboard-id",
		Name: "dashboard",
		Items: []fastly.DashboardItem{
			remoteItem("api-b", "Errors", "status_5xx"),
			remoteItem("api-a", "Requests", "requests"),
		},
	}

	flattenDashboard(&state, remote)

	require.Len(t, state.DashboardItem, 2)
	require.Equal(t, "requests", state.DashboardItem[0].Key.ValueString())
	require.Equal(t, "api-a", state.DashboardItem[0].ID.ValueString())
	require.Equal(t, "errors", state.DashboardItem[1].Key.ValueString())
	require.Equal(t, "api-b", state.DashboardItem[1].ID.ValueString())
}

func TestFlattenDashboardAssociatesNewFastlyIDWithDesiredKey(t *testing.T) {
	model := testDashboardModel()

	remote := &fastly.ObservabilityCustomDashboard{
		ID:   "dashboard-id",
		Name: "dashboard",
		Items: []fastly.DashboardItem{
			remoteItemFromModel(t, model.DashboardItem[0], "generated-id"),
		},
	}

	flattenDashboard(&model, remote)

	require.Len(t, model.DashboardItem, 1)
	require.Equal(t, "requests", model.DashboardItem[0].Key.ValueString())
	require.Equal(t, "generated-id", model.DashboardItem[0].ID.ValueString())
}

func TestFlattenDashboardImportSynthesizesKeyFromFastlyID(t *testing.T) {
	var state Model
	remote := &fastly.ObservabilityCustomDashboard{
		ID:   "dashboard-id",
		Name: "dashboard",
		Items: []fastly.DashboardItem{
			remoteItem("generated-id", "Requests", "requests"),
		},
	}

	flattenDashboard(&state, remote)

	require.Len(t, state.DashboardItem, 1)
	require.Equal(t, "generated-id", state.DashboardItem[0].Key.ValueString())
	require.Equal(t, "generated-id", state.DashboardItem[0].ID.ValueString())
}

func TestFlattenDashboardKeepsOmittedDescriptionNull(t *testing.T) {
	state := testDashboardModel()
	state.Description = types.StringNull()
	remote := &fastly.ObservabilityCustomDashboard{
		ID:          "dashboard-id",
		Name:        "dashboard",
		Description: "",
		Items:       nil,
	}

	flattenDashboard(&state, remote)

	require.Equal(t, types.StringValue("dashboard-id"), state.ID)
	require.True(t, state.Description.IsNull())
	require.Empty(t, state.DashboardItem)
}

func TestFlattenDashboardPreservesExplicitEmptyDescription(t *testing.T) {
	state := testDashboardModel()
	state.Description = types.StringValue("")
	remote := &fastly.ObservabilityCustomDashboard{
		ID:          "dashboard-id",
		Name:        "dashboard",
		Description: "",
		Items:       nil,
	}

	flattenDashboard(&state, remote)

	require.Equal(t, types.StringValue(""), state.Description)
}

func TestValidateUniqueKeys(t *testing.T) {
	items := []DashboardItemModel{
		{Key: types.StringValue("requests")},
		{Key: types.StringValue("requests")},
	}

	diags := validateUniqueKeys(items)
	require.True(t, diags.HasError())
}

func remoteItemFromModel(t *testing.T, model DashboardItemModel, id string) fastly.DashboardItem {
	t.Helper()

	items, diags := expandItems(context.Background(), []DashboardItemModel{model})
	require.False(t, diags.HasError(), diags)
	require.Len(t, items, 1)

	item := items[0]
	item.ID = id
	return item
}

func remoteItem(id, title, metric string) fastly.DashboardItem {
	format := fastly.VisualizationFormat("number")
	return fastly.DashboardItem{
		ID:       id,
		Title:    title,
		Subtitle: "subtitle",
		Span:     4,
		DataSource: fastly.DashboardDataSource{
			Type: fastly.DashboardSourceType("stats.edge"),
			Config: fastly.DashboardSourceConfig{
				Metrics: []string{metric},
			},
		},
		Visualization: fastly.DashboardVisualization{
			Type: fastly.VisualizationType("chart"),
			Config: fastly.VisualizationConfig{
				PlotType: fastly.PlotType("bar"),
				Format:   &format,
			},
		},
	}
}
