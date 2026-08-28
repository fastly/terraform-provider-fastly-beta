package ngwafworkspace

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
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
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Fastly Next-Gen WAF workspace. Workspaces are versionless and independent of any service-version lifecycle.",
		Attributes:  ResourceAttributes(),
		Blocks: map[string]schema.Block{
			"attack_signal_thresholds": AttackSignalThresholdsBlock(),
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

	input, diags := BuildCreateInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly NGWAF workspace", map[string]any{"name": plan.Name.ValueString()})

	workspace, err := ws.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating NGWAF workspace", err.Error())
		return
	}

	state, diags := FlattenToModel(ctx, workspace)
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

	workspaceID := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly NGWAF workspace", map[string]any{"id": workspaceID})

	workspace, err := ws.Get(ctx, r.client, &ws.GetInput{WorkspaceID: &workspaceID})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF workspace not found, removing from state", map[string]any{"id": workspaceID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF workspace", err.Error())
		return
	}

	newState, diags := FlattenToModel(ctx, workspace)
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

	workspaceID := state.ID.ValueString()

	input, diags := BuildUpdateInput(ctx, workspaceID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly NGWAF workspace", map[string]any{"id": workspaceID})

	workspace, err := ws.Update(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating NGWAF workspace", err.Error())
		return
	}

	newState, diags := FlattenToModel(ctx, workspace)
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

	workspaceID := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly NGWAF workspace", map[string]any{"id": workspaceID})

	if err := ws.Delete(ctx, r.client, &ws.DeleteInput{WorkspaceID: &workspaceID}); err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting NGWAF workspace", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
