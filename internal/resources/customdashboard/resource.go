package customdashboard

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/errors"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_dashboard"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Fastly custom Observability dashboard. Custom dashboards are versionless and independent of service-version lifecycle.",
		Attributes:  ResourceAttributes(),
		Blocks: map[string]schema.Block{
			"dashboard_item": DashboardItemsBlock(),
		},
	}
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}
	r.client = data.Client
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateUniqueKeys(plan.DashboardItem)...)
	if resp.Diagnostics.HasError() {
		return
	}

	items, diags := expandItems(ctx, plan.DashboardItem)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := &fastly.CreateObservabilityCustomDashboardInput{
		Name:  plan.Name.ValueString(),
		Items: items,
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = new(plan.Description.ValueString())
	}

	tflog.Debug(ctx, "Creating Fastly custom dashboard", map[string]any{"name": plan.Name.ValueString()})

	dashboard, err := r.client.CreateObservabilityCustomDashboard(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating custom dashboard", err.Error())
		return
	}

	// plan contains the Terraform-owned keys. The API response contains the
	// Fastly-owned item IDs. flattenDashboard associates the two without ever
	// requiring the configured key to be sent to Fastly.
	flattenDashboard(&plan, dashboard)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboard, err := r.client.GetObservabilityCustomDashboard(ctx, &fastly.GetObservabilityCustomDashboardInput{
		ID: new(state.ID.ValueString()),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Custom dashboard not found, removing from state", map[string]any{"id": state.ID.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading custom dashboard", err.Error())
		return
	}

	// Existing API IDs preserve configured keys even if Fastly returns items in a
	// different order. During import there are no prior keys, so the API item ID
	// becomes a deterministic initial key.
	flattenDashboard(&state, dashboard)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	var state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateUniqueKeys(plan.DashboardItem)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The configured key is Terraform identity. Re-bind Fastly's computed item
	// IDs by key from prior state so list reordering cannot accidentally attach
	// an old API ID to the item now occupying that list index.
	bindIDsByKey(plan.DashboardItem, state.DashboardItem)
	plan.ID = state.ID

	items, diags := expandItems(ctx, plan.DashboardItem)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	name := plan.Name.ValueString()
	description := ""
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		description = plan.Description.ValueString()
	}

	input := &fastly.UpdateObservabilityCustomDashboardInput{
		ID:          &id,
		Name:        &name,
		Description: &description,
		Items:       &items,
	}

	dashboard, err := r.client.UpdateObservabilityCustomDashboard(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating custom dashboard", err.Error())
		return
	}

	flattenDashboard(&plan, dashboard)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteObservabilityCustomDashboard(ctx, &fastly.DeleteObservabilityCustomDashboardInput{
		ID: new(state.ID.ValueString()),
	})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting custom dashboard", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Read synthesizes each dashboard_item.key from the Fastly item ID when no
	// prior Terraform key exists. This keeps imported state deterministic while
	// preserving the distinction between Terraform identity and API identity.
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func validateUniqueKeys(items []DashboardItemModel) diag.Diagnostics {
	var diags diag.Diagnostics
	seen := make(map[string]int, len(items))

	for i, item := range items {
		if item.Key.IsNull() || item.Key.IsUnknown() {
			continue
		}
		key := item.Key.ValueString()
		if first, ok := seen[key]; ok {
			diags.AddAttributeError(
				path.Root("dashboard_item").AtListIndex(i).AtName("key"),
				"Duplicate dashboard item key",
				"Dashboard item key "+key+" is already used by dashboard_item at index "+itoa(first)+". Each dashboard item key must be unique within the dashboard.",
			)
			continue
		}
		seen[key] = i
	}
	return diags
}

// itoa is intentionally tiny to keep the validation helper dependency-free.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func bindIDsByKey(desired, previous []DashboardItemModel) {
	usedPrevious := make([]bool, len(previous))

	// First preserve the normal configured-key identity contract.
	for i := range desired {
		if desired[i].Key.IsNull() || desired[i].Key.IsUnknown() {
			desired[i].ID = types.StringNull()
			continue
		}

		matched := false
		for j := range previous {
			if usedPrevious[j] ||
				previous[j].Key.IsNull() || previous[j].Key.IsUnknown() ||
				previous[j].ID.IsNull() || previous[j].ID.IsUnknown() ||
				previous[j].ID.ValueString() == "" {
				continue
			}
			if previous[j].Key.ValueString() == desired[i].Key.ValueString() {
				desired[i].ID = previous[j].ID
				usedPrevious[j] = true
				matched = true
				break
			}
		}
		if !matched {
			desired[i].ID = types.StringNull()
		}
	}

	// Import has no Terraform-owned keys to recover, so Read synthesizes key=id.
	// On the first apply after import, allow a friendly configured key to adopt
	// that existing Fastly item only when there is exactly one equivalent
	// synthetic item. This preserves the Fastly ID without weakening the normal
	// rule that changing an established configured key creates a new item.
	for i := range desired {
		if !desired[i].ID.IsNull() || desired[i].Key.IsNull() || desired[i].Key.IsUnknown() {
			continue
		}

		candidate := -1
		for j := range previous {
			if usedPrevious[j] || !isSyntheticImportedIdentity(previous[j]) {
				continue
			}
			if dashboardItemModelsEquivalent(desired[i], previous[j]) {
				if candidate != -1 {
					candidate = -2 // ambiguous: do not guess
					break
				}
				candidate = j
			}
		}
		if candidate >= 0 {
			desired[i].ID = previous[candidate].ID
			usedPrevious[candidate] = true
		}
	}
}

func isSyntheticImportedIdentity(item DashboardItemModel) bool {
	if item.Key.IsNull() || item.Key.IsUnknown() || item.ID.IsNull() || item.ID.IsUnknown() {
		return false
	}
	id := item.ID.ValueString()
	return id != "" && item.Key.ValueString() == id
}

func dashboardItemModelsEquivalent(a, b DashboardItemModel) bool {
	if a.Title.ValueString() != b.Title.ValueString() ||
		a.Subtitle.ValueString() != b.Subtitle.ValueString() ||
		a.Span.ValueInt64() != b.Span.ValueInt64() ||
		len(a.DataSource) != 1 || len(b.DataSource) != 1 ||
		len(a.DataSource[0].Config) != 1 || len(b.DataSource[0].Config) != 1 ||
		len(a.Visualization) != 1 || len(b.Visualization) != 1 ||
		len(a.Visualization[0].Config) != 1 || len(b.Visualization[0].Config) != 1 {
		return false
	}

	if a.DataSource[0].Type.ValueString() != b.DataSource[0].Type.ValueString() ||
		a.Visualization[0].Type.ValueString() != b.Visualization[0].Type.ValueString() ||
		a.Visualization[0].Config[0].PlotType.ValueString() != b.Visualization[0].Config[0].PlotType.ValueString() {
		return false
	}

	var aMetrics, bMetrics []string
	if diags := a.DataSource[0].Config[0].Metrics.ElementsAs(context.Background(), &aMetrics, false); diags.HasError() {
		return false
	}
	if diags := b.DataSource[0].Config[0].Metrics.ElementsAs(context.Background(), &bMetrics, false); diags.HasError() {
		return false
	}
	if len(aMetrics) != len(bMetrics) {
		return false
	}
	for i := range aMetrics {
		if aMetrics[i] != bMetrics[i] {
			return false
		}
	}

	aCalc := ""
	if !a.Visualization[0].Config[0].CalculationMethod.IsNull() && !a.Visualization[0].Config[0].CalculationMethod.IsUnknown() {
		aCalc = a.Visualization[0].Config[0].CalculationMethod.ValueString()
	}
	bCalc := ""
	if !b.Visualization[0].Config[0].CalculationMethod.IsNull() && !b.Visualization[0].Config[0].CalculationMethod.IsUnknown() {
		bCalc = b.Visualization[0].Config[0].CalculationMethod.ValueString()
	}
	if aCalc != bCalc {
		return false
	}

	aFormat := DefaultVisualizationFormat
	if !a.Visualization[0].Config[0].Format.IsNull() && !a.Visualization[0].Config[0].Format.IsUnknown() && a.Visualization[0].Config[0].Format.ValueString() != "" {
		aFormat = a.Visualization[0].Config[0].Format.ValueString()
	}
	bFormat := DefaultVisualizationFormat
	if !b.Visualization[0].Config[0].Format.IsNull() && !b.Visualization[0].Config[0].Format.IsUnknown() && b.Visualization[0].Config[0].Format.ValueString() != "" {
		bFormat = b.Visualization[0].Config[0].Format.ValueString()
	}
	return aFormat == bFormat
}

func flattenDashboard(state *Model, dashboard *fastly.ObservabilityCustomDashboard) {
	previous := state.DashboardItem
	previousDescription := state.Description

	state.ID = types.StringValue(dashboard.ID)
	state.Name = types.StringValue(dashboard.Name)
	if dashboard.Description == "" {
		// The API uses the same empty string for both an omitted optional
		// description and an explicitly configured empty description. Preserve the
		// Terraform representation so either form converges without drift.
		if previousDescription.IsNull() || previousDescription.IsUnknown() {
			state.Description = types.StringNull()
		} else {
			state.Description = types.StringValue("")
		}
	} else {
		state.Description = types.StringValue(dashboard.Description)
	}
	state.DashboardItem = flattenItems(dashboard.Items, previous)
}

func flattenItems(items []fastly.DashboardItem, previous []DashboardItemModel) []DashboardItemModel {
	if len(items) == 0 {
		return nil
	}

	keyByID := make(map[string]string, len(previous))
	for _, item := range previous {
		if item.Key.IsNull() || item.Key.IsUnknown() || item.ID.IsNull() || item.ID.IsUnknown() || item.ID.ValueString() == "" {
			continue
		}
		keyByID[item.ID.ValueString()] = item.Key.ValueString()
	}

	usedPrevious := make([]bool, len(previous))
	result := make([]DashboardItemModel, 0, len(items))

	for _, item := range items {
		key, known := keyByID[item.ID]
		if known {
			for i := range previous {
				if !previous[i].ID.IsNull() && !previous[i].ID.IsUnknown() && previous[i].ID.ValueString() == item.ID {
					usedPrevious[i] = true
					break
				}
			}
		} else {
			// Create/new-item responses do not have a previous API ID to match.
			// Match the returned content to the desired item so the Terraform key
			// survives even if the API changes item ordering.
			for i := range previous {
				if usedPrevious[i] || (!previous[i].ID.IsNull() && !previous[i].ID.IsUnknown() && previous[i].ID.ValueString() != "") {
					continue
				}
				if modelMatchesRemote(previous[i], item) {
					key = previous[i].Key.ValueString()
					usedPrevious[i] = true
					known = true
					break
				}
			}
		}
		if !known {
			// Import or an out-of-band item: use the API ID as deterministic
			// Terraform identity until the user deliberately adopts another key.
			key = item.ID
		}

		metrics := make([]string, len(item.DataSource.Config.Metrics))
		copy(metrics, item.DataSource.Config.Metrics)
		metricsValue, _ := types.ListValueFrom(context.Background(), types.StringType, metrics)

		vizConfig := VisualizationConfigModel{
			PlotType: types.StringValue(string(item.Visualization.Config.PlotType)),
			Format:   types.StringValue(DefaultVisualizationFormat),
		}
		if item.Visualization.Config.CalculationMethod != nil {
			vizConfig.CalculationMethod = types.StringValue(string(*item.Visualization.Config.CalculationMethod))
		} else {
			vizConfig.CalculationMethod = types.StringNull()
		}
		if item.Visualization.Config.Format != nil && *item.Visualization.Config.Format != "" {
			vizConfig.Format = types.StringValue(string(*item.Visualization.Config.Format))
		}

		result = append(result, DashboardItemModel{
			Key:      types.StringValue(key),
			ID:       types.StringValue(item.ID),
			Title:    types.StringValue(item.Title),
			Subtitle: types.StringValue(item.Subtitle),
			Span:     types.Int64Value(int64(item.Span)),
			DataSource: []DataSourceModel{{
				Type: types.StringValue(string(item.DataSource.Type)),
				Config: []DataSourceConfigModel{{
					Metrics: metricsValue,
				}},
			}},
			Visualization: []VisualizationModel{{
				Type:   types.StringValue(string(item.Visualization.Type)),
				Config: []VisualizationConfigModel{vizConfig},
			}},
		})
	}
	return result
}

func modelMatchesRemote(model DashboardItemModel, remote fastly.DashboardItem) bool {
	if model.Key.IsNull() || model.Key.IsUnknown() {
		return false
	}
	if model.Title.ValueString() != remote.Title ||
		model.Subtitle.ValueString() != remote.Subtitle ||
		model.Span.ValueInt64() != int64(remote.Span) ||
		len(model.DataSource) != 1 ||
		len(model.DataSource[0].Config) != 1 ||
		len(model.Visualization) != 1 ||
		len(model.Visualization[0].Config) != 1 {
		return false
	}

	if model.DataSource[0].Type.ValueString() != string(remote.DataSource.Type) ||
		model.Visualization[0].Type.ValueString() != string(remote.Visualization.Type) ||
		model.Visualization[0].Config[0].PlotType.ValueString() != string(remote.Visualization.Config.PlotType) {
		return false
	}

	var metrics []string
	if diags := model.DataSource[0].Config[0].Metrics.ElementsAs(context.Background(), &metrics, false); diags.HasError() {
		return false
	}
	if len(metrics) != len(remote.DataSource.Config.Metrics) {
		return false
	}
	for i := range metrics {
		if metrics[i] != remote.DataSource.Config.Metrics[i] {
			return false
		}
	}

	modelCalc := ""
	if !model.Visualization[0].Config[0].CalculationMethod.IsNull() && !model.Visualization[0].Config[0].CalculationMethod.IsUnknown() {
		modelCalc = model.Visualization[0].Config[0].CalculationMethod.ValueString()
	}
	remoteCalc := ""
	if remote.Visualization.Config.CalculationMethod != nil {
		remoteCalc = string(*remote.Visualization.Config.CalculationMethod)
	}
	if modelCalc != remoteCalc {
		return false
	}

	modelFormat := DefaultVisualizationFormat
	if !model.Visualization[0].Config[0].Format.IsNull() && !model.Visualization[0].Config[0].Format.IsUnknown() && model.Visualization[0].Config[0].Format.ValueString() != "" {
		modelFormat = model.Visualization[0].Config[0].Format.ValueString()
	}
	remoteFormat := DefaultVisualizationFormat
	if remote.Visualization.Config.Format != nil && *remote.Visualization.Config.Format != "" {
		remoteFormat = string(*remote.Visualization.Config.Format)
	}
	return modelFormat == remoteFormat
}

func expandItems(ctx context.Context, items []DashboardItemModel) ([]fastly.DashboardItem, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make([]fastly.DashboardItem, 0, len(items))

	for i, item := range items {
		if len(item.DataSource) != 1 || len(item.DataSource[0].Config) != 1 {
			diags.AddAttributeError(
				path.Root("dashboard_item").AtListIndex(i).AtName("data_source"),
				"Invalid dashboard data source",
				"Exactly one data_source block containing exactly one config block is required.",
			)
			continue
		}
		if len(item.Visualization) != 1 || len(item.Visualization[0].Config) != 1 {
			diags.AddAttributeError(
				path.Root("dashboard_item").AtListIndex(i).AtName("visualization"),
				"Invalid dashboard visualization",
				"Exactly one visualization block containing exactly one config block is required.",
			)
			continue
		}

		var metrics []string
		diags.Append(item.DataSource[0].Config[0].Metrics.ElementsAs(ctx, &metrics, false)...)
		if diags.HasError() {
			continue
		}

		viz := item.Visualization[0].Config[0]
		apiItem := fastly.DashboardItem{
			Title:    item.Title.ValueString(),
			Subtitle: item.Subtitle.ValueString(),
			Span:     uint8(item.Span.ValueInt64()),
			DataSource: fastly.DashboardDataSource{
				Type: fastly.DashboardSourceType(item.DataSource[0].Type.ValueString()),
				Config: fastly.DashboardSourceConfig{
					Metrics: metrics,
				},
			},
			Visualization: fastly.DashboardVisualization{
				Type: fastly.VisualizationType(item.Visualization[0].Type.ValueString()),
				Config: fastly.VisualizationConfig{
					PlotType: fastly.PlotType(viz.PlotType.ValueString()),
				},
			},
		}

		// Fastly owns dashboard-item IDs. Existing items get their API ID rebound
		// from state by key during Update; new items and all Create items omit ID.
		if !item.ID.IsNull() && !item.ID.IsUnknown() && item.ID.ValueString() != "" {
			apiItem.ID = item.ID.ValueString()
		}

		if !viz.CalculationMethod.IsNull() && !viz.CalculationMethod.IsUnknown() && viz.CalculationMethod.ValueString() != "" {
			v := fastly.CalculationMethod(viz.CalculationMethod.ValueString())
			apiItem.Visualization.Config.CalculationMethod = &v
		}
		format := DefaultVisualizationFormat
		if !viz.Format.IsNull() && !viz.Format.IsUnknown() && viz.Format.ValueString() != "" {
			format = viz.Format.ValueString()
		}
		v := fastly.VisualizationFormat(format)
		apiItem.Visualization.Config.Format = &v

		result = append(result, apiItem)
	}

	return result, diags
}
