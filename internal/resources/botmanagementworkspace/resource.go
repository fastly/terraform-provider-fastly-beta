package botmanagementworkspace

import (
	"context"
	"fmt"
	"strings"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"
	ws "github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces"
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithConfigure   = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
	_ resource.ResourceWithIdentity    = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bot_management_workspace"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the settings of an existing Fastly Bot Management (ContentGuard) workspace. A Bot Management workspace is created when ContentGuard is enabled on a service. This resource doesn't create or delete the workspace, and destroying the resource only removes it from Terraform state.",
		Attributes:  ResourceAttributes(),
	}
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = resourceidentity.BotManagementWorkspaceSchema()
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

	tflog.Debug(ctx, "Adopting Fastly Bot Management workspace", map[string]any{"workspace_id": plan.WorkspaceID.ValueString()})
	r.write(ctx, plan, &resp.State, resp.Identity, &resp.Diagnostics, "Error updating Bot Management workspace")
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	tflog.Debug(ctx, "Reading Fastly Bot Management workspace", map[string]any{"workspace_id": workspaceID})

	workspace, err := ws.Get(ctx, r.client, &ws.GetInput{WorkspaceID: &workspaceID})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Bot Management workspace not found, removing from state", map[string]any{"workspace_id": workspaceID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Bot Management workspace", err.Error())
		return
	}

	newState := FlattenToModel(workspace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
	resourceidentity.Set(ctx, resp.Identity, &resp.Diagnostics, resourceidentity.BotManagementWorkspace(newState.WorkspaceID))
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly Bot Management workspace", map[string]any{"workspace_id": plan.WorkspaceID.ValueString()})
	r.write(ctx, plan, &resp.State, resp.Identity, &resp.Diagnostics, "Error updating Bot Management workspace")
}

// Delete only removes the workspace from state. The API has no delete
// endpoint, and there's no neutral protection_mode to reset to.
func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, _ *resource.DeleteResponse) {
	var state Model
	if diags := req.State.Get(ctx, &state); !diags.HasError() {
		tflog.Debug(ctx, "Removing Fastly Bot Management workspace from state; its settings are left unchanged", map[string]any{"workspace_id": state.WorkspaceID.ValueString()})
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resourceidentity.ImportBotManagementWorkspace(ctx, req, resp)
		return
	}

	if strings.TrimSpace(req.ID) != req.ID || strings.Contains(req.ID, "/") {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected import identifier of the form \"workspace_id\", got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), req.ID)...)
}

func (r *Resource) write(ctx context.Context, plan Model, state *tfsdk.State, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics, summary string) {
	workspace, err := ws.Update(ctx, r.client, BuildUpdateInput(plan))
	if err != nil {
		diags.AddError(summary, err.Error())
		return
	}

	newState := FlattenToModel(workspace)
	diags.Append(state.Set(ctx, &newState)...)
	resourceidentity.Set(ctx, identity, diags, resourceidentity.BotManagementWorkspace(newState.WorkspaceID))
}
