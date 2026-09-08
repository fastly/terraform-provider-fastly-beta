package integration

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
)

const mailingListConfirmationNotice = "Please visit https://manage.fastly.com/observability/alerts/integrations to send a confirmation email and/or verify status."

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a Fastly Integration, a custom notification channel (e.g. Slack, PagerDuty, Datadog) that a `fastly_alert` can notify when it fires. Integrations are versionless and independent of any service-version lifecycle.",
		Attributes:  ResourceAttributes(),
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

	input, diags := BuildCreateInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly Integration", map[string]any{"name": plan.Name.ValueString()})

	created, err := r.client.CreateIntegration(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Integration", err.Error())
		return
	}

	i, err := r.client.GetIntegration(ctx, &fastly.GetIntegrationInput{ID: fastly.ToValue(created.ID)})
	if err != nil {
		resp.Diagnostics.AddError("Error reading newly created Integration", err.Error())
		return
	}

	state, diags := FlattenToModel(ctx, i, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	warnIfMailingListUnconfirmed(&resp.Diagnostics, i)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()

	tflog.Debug(ctx, "Reading Fastly Integration", map[string]any{"id": id})

	i, err := r.client.GetIntegration(ctx, &fastly.GetIntegrationInput{ID: id})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Integration not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading Integration", err.Error())
		return
	}

	newState, diags := FlattenToModel(ctx, i, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	warnIfMailingListUnconfirmed(&resp.Diagnostics, i)

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

	id := state.ID.ValueString()

	input, diags := BuildUpdateInput(ctx, id, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly Integration", map[string]any{"id": id})

	if err := r.client.UpdateIntegration(ctx, input); err != nil {
		resp.Diagnostics.AddError("Error updating Integration", err.Error())
		return
	}

	i, err := r.client.GetIntegration(ctx, &fastly.GetIntegrationInput{ID: id})
	if err != nil {
		resp.Diagnostics.AddError("Error reading updated Integration", err.Error())
		return
	}

	newState, diags := FlattenToModel(ctx, i, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	warnIfMailingListUnconfirmed(&resp.Diagnostics, i)

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly Integration", map[string]any{"id": id})

	err := r.client.DeleteIntegration(ctx, &fastly.DeleteIntegrationInput{ID: id})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Integration", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// warnIfMailingListUnconfirmed mirrors the legacy provider's confirmation nudge.
func warnIfMailingListUnconfirmed(diags *diag.Diagnostics, i *fastly.Integration) {
	if fastly.ToValue(i.Type) != typeMailingList {
		return
	}
	if fastly.ToValue(i.Status) == "confirmed" {
		return
	}

	diags.AddWarning("Mailing list integration needs confirmation.", mailingListConfirmationNotice)
}
