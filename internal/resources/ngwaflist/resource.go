package ngwaflist

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithConfigure   = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

// Resource implements a type-specific workspace NGWAF list resource.
type Resource struct {
	client      *fastly.Client
	listType    string
	typeSuffix  string
	description string
}

// NewWorkspaceResource returns a workspace-scoped NGWAF list resource for one
// concrete API list type.
func NewWorkspaceResource(listType, typeSuffix, description string) resource.Resource {
	return &Resource{
		listType:    listType,
		typeSuffix:  typeSuffix,
		description: description,
	}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_" + r.typeSuffix + "_list"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: r.description,
		Attributes:  Attributes(r.listType),
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

	input, diags := BuildCreateInput(ctx, r.listType, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly NGWAF workspace list", map[string]any{
		"workspace_id": plan.WorkspaceID.ValueString(),
		"type":         r.listType,
		"name":         plan.Name.ValueString(),
	})

	list, err := lists.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error creating NGWAF workspace %s list", r.listType), err.Error())
		return
	}

	state, err := FlattenToModel(r.listType, plan.WorkspaceID.ValueString(), list)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error reading NGWAF workspace %s list", r.listType), err.Error())
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
	listID := state.ID.ValueString()

	tflog.Debug(ctx, "Reading Fastly NGWAF workspace list", map[string]any{
		"workspace_id": workspaceID,
		"id":           listID,
		"type":         r.listType,
	})

	list, err := lists.Get(ctx, r.client, BuildGetInput(workspaceID, listID))
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF workspace list not found, removing from state", map[string]any{
				"workspace_id": workspaceID,
				"id":           listID,
				"type":         r.listType,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("Error reading NGWAF workspace %s list", r.listType), err.Error())
		return
	}

	newState, err := FlattenToModel(r.listType, workspaceID, list)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error reading NGWAF workspace %s list", r.listType), err.Error())
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

	listID := state.ID.ValueString()

	input, diags := BuildUpdateInput(ctx, listID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly NGWAF workspace list", map[string]any{
		"workspace_id": plan.WorkspaceID.ValueString(),
		"id":           listID,
		"type":         r.listType,
	})

	list, err := lists.Update(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error updating NGWAF workspace %s list", r.listType), err.Error())
		return
	}

	newState, err := FlattenToModel(r.listType, plan.WorkspaceID.ValueString(), list)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error reading NGWAF workspace %s list", r.listType), err.Error())
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
	listID := state.ID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly NGWAF workspace list", map[string]any{
		"workspace_id": workspaceID,
		"id":           listID,
		"type":         r.listType,
	})

	err := lists.Delete(ctx, r.client, BuildDeleteInput(workspaceID, listID))
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError(fmt.Sprintf("Error deleting NGWAF workspace %s list", r.listType), err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	ImportState(ctx, req, resp)
}
