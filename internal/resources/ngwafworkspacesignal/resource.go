package ngwafworkspacesignal

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithConfigure   = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_signal"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Fastly Next-Gen WAF custom signal scoped to a single workspace.",
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

	input := BuildCreateInput(plan)

	tflog.Debug(ctx, "Creating Fastly NGWAF workspace signal", map[string]any{
		"workspace_id": plan.WorkspaceID.ValueString(),
		"name":         plan.Name.ValueString(),
	})

	signal, err := signals.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating NGWAF workspace signal", err.Error())
		return
	}

	state, err := FlattenToModel(signal)
	if err != nil {
		resp.Diagnostics.AddError("Error reading NGWAF workspace signal", err.Error())
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

	workspaceID := state.WorkspaceID.ValueString()
	signalID := state.ID.ValueString()

	tflog.Debug(ctx, "Reading Fastly NGWAF workspace signal", map[string]any{
		"workspace_id": workspaceID,
		"id":           signalID,
	})

	signal, err := signals.Get(ctx, r.client, BuildGetInput(workspaceID, signalID))
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF workspace signal not found, removing from state", map[string]any{
				"workspace_id": workspaceID,
				"id":           signalID,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF workspace signal", err.Error())
		return
	}

	newState, err := FlattenToModel(signal)
	if err != nil {
		resp.Diagnostics.AddError("Error reading NGWAF workspace signal", err.Error())
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

	signalID := state.ID.ValueString()

	tflog.Debug(ctx, "Updating Fastly NGWAF workspace signal", map[string]any{
		"workspace_id": plan.WorkspaceID.ValueString(),
		"id":           signalID,
	})

	signal, err := signals.Update(ctx, r.client, BuildUpdateInput(signalID, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating NGWAF workspace signal", err.Error())
		return
	}

	newState, err := FlattenToModel(signal)
	if err != nil {
		resp.Diagnostics.AddError("Error reading NGWAF workspace signal", err.Error())
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

	workspaceID := state.WorkspaceID.ValueString()
	signalID := state.ID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly NGWAF workspace signal", map[string]any{
		"workspace_id": workspaceID,
		"id":           signalID,
	})

	err := signals.Delete(ctx, r.client, BuildDeleteInput(workspaceID, signalID))
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting NGWAF workspace signal", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	workspaceID, signalID, err := ParseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), workspaceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), signalID)...)
}
