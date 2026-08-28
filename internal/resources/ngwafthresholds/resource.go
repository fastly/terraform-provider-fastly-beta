package ngwafthresholds

import (
	"context"
	"fmt"
	"strings"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	th "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/thresholds"
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
	resp.TypeName = req.ProviderTypeName + "_ngwaf_thresholds"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Fastly Next-Gen WAF threshold within a workspace.",
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

	tflog.Debug(ctx, "Creating Fastly NGWAF threshold", map[string]any{"workspace_id": plan.WorkspaceID.ValueString(), "name": plan.Name.ValueString()})

	threshold, err := th.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating NGWAF threshold", err.Error())
		return
	}

	state := FlattenToModel(plan.WorkspaceID.ValueString(), threshold)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	thresholdID := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly NGWAF threshold", map[string]any{"workspace_id": workspaceID, "id": thresholdID})

	threshold, err := th.Get(ctx, r.client, &th.GetInput{WorkspaceID: &workspaceID, ThresholdID: &thresholdID})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF threshold not found, removing from state", map[string]any{"id": thresholdID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF threshold", err.Error())
		return
	}

	newState := FlattenToModel(workspaceID, threshold)

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

	workspaceID := state.WorkspaceID.ValueString()
	thresholdID := state.ID.ValueString()

	input := BuildUpdateInput(workspaceID, thresholdID, plan)

	tflog.Debug(ctx, "Updating Fastly NGWAF threshold", map[string]any{"workspace_id": workspaceID, "id": thresholdID})

	threshold, err := th.Update(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating NGWAF threshold", err.Error())
		return
	}

	newState := FlattenToModel(workspaceID, threshold)

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	thresholdID := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly NGWAF threshold", map[string]any{"workspace_id": workspaceID, "id": thresholdID})

	if err := th.Delete(ctx, r.client, &th.DeleteInput{WorkspaceID: &workspaceID, ThresholdID: &thresholdID}); err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting NGWAF threshold", err.Error())
	}
}

// ImportState populates workspace_id and id from a "workspace_id/threshold_id"
// import identifier, matching the legacy provider's import format.
func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected import identifier of the form \"workspace_id/threshold_id\", got: %q", req.ID),
		)
		return
	}
	workspaceID, thresholdID := parts[0], parts[1]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), workspaceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), thresholdID)...)
}
