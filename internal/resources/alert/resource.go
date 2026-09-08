package alert

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
)

var (
	_ resource.Resource                   = &Resource{}
	_ resource.ResourceWithConfigure      = &Resource{}
	_ resource.ResourceWithValidateConfig = &Resource{}
	_ resource.ResourceWithImportState    = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a Fastly Alert. Alerts send notifications to custom integrations (e.g., Slack channels, PagerDuty, Microsoft Teams and New Relic) when an observed metric either exceeds or falls below a threshold. Alerts are versionless and independent of any service-version lifecycle.",
		Attributes:  ResourceAttributes(),
		Blocks: map[string]schema.Block{
			"dimensions":          DimensionsBlock(),
			"evaluation_strategy": EvaluationStrategyBlock(),
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

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Source.IsUnknown() || config.ServiceID.IsUnknown() {
		return
	}

	if err := validateSourceWithServiceID(config.Source.ValueString(), config.ServiceID.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("service_id"),
			"Invalid Alert Configuration",
			err.Error(),
		)
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input, diags := BuildCreateInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly Alert", map[string]any{"name": plan.Name.ValueString()})

	ad, err := r.client.CreateAlertDefinition(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Alert", err.Error())
		return
	}

	state, diags := FlattenToModel(ctx, ad)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alertID := state.ID.ValueString()

	tflog.Debug(ctx, "Reading Fastly Alert", map[string]any{"id": alertID})

	ad, err := r.client.GetAlertDefinition(ctx, &fastly.GetAlertDefinitionInput{ID: &alertID})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Alert not found, removing from state", map[string]any{"id": alertID})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading Alert", err.Error())
		return
	}

	newState, diags := FlattenToModel(ctx, ad)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	var state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alertID := state.ID.ValueString()

	input, diags := BuildUpdateInput(ctx, alertID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly Alert", map[string]any{"id": alertID})

	ad, err := r.client.UpdateAlertDefinition(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Alert", err.Error())
		return
	}

	newState, diags := FlattenToModel(ctx, ad)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alertID := state.ID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly Alert", map[string]any{"id": alertID})

	if err := r.client.DeleteAlertDefinition(ctx, &fastly.DeleteAlertDefinitionInput{ID: &alertID}); err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Alert", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
