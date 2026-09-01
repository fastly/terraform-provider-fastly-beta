package ngwafworkspacevirtualpatch

import (
	"context"
	"fmt"
	"strings"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	vp "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/virtualpatches"
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
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_virtual_patch"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an existing Fastly Next-Gen WAF virtual patch within a workspace. Virtual patches are created by Fastly; this resource configures the action and enabled state for an existing patch.",
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

	workspaceID := plan.WorkspaceID.ValueString()
	virtualPatchID := plan.VirtualPatchID.ValueString()

	tflog.Debug(ctx, "Configuring Fastly NGWAF workspace virtual patch", map[string]any{"workspace_id": workspaceID, "virtual_patch_id": virtualPatchID})

	existing, err := vp.Get(ctx, r.client, BuildGetInput(workspaceID, virtualPatchID))
	if err != nil {
		if errors.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"NGWAF virtual patch not found",
				fmt.Sprintf("Virtual patches cannot be created by Terraform. Use this resource to configure an existing virtual patch. Virtual patch %q does not exist in workspace %q.", virtualPatchID, workspaceID),
			)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF virtual patch before configure", err.Error())
		return
	}
	if existing == nil {
		resp.Diagnostics.AddError(
			"NGWAF virtual patch not found",
			fmt.Sprintf("Virtual patches cannot be created by Terraform. Use this resource to configure an existing virtual patch. Virtual patch %q does not exist in workspace %q.", virtualPatchID, workspaceID),
		)
		return
	}

	virtualPatch, err := vp.Update(ctx, r.client, BuildUpdateInput(workspaceID, virtualPatchID, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error configuring NGWAF virtual patch", err.Error())
		return
	}

	state, err := FlattenToModel(workspaceID, virtualPatch)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening NGWAF virtual patch", err.Error())
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
	virtualPatchID := state.VirtualPatchID.ValueString()

	tflog.Debug(ctx, "Reading Fastly NGWAF workspace virtual patch", map[string]any{"workspace_id": workspaceID, "virtual_patch_id": virtualPatchID})

	virtualPatch, err := vp.Get(ctx, r.client, BuildGetInput(workspaceID, virtualPatchID))
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF workspace virtual patch not found, removing from state", map[string]any{"virtual_patch_id": virtualPatchID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF virtual patch", err.Error())
		return
	}

	newState, err := FlattenToModel(workspaceID, virtualPatch)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening NGWAF virtual patch", err.Error())
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

	workspaceID := state.WorkspaceID.ValueString()
	virtualPatchID := state.VirtualPatchID.ValueString()

	tflog.Debug(ctx, "Updating Fastly NGWAF workspace virtual patch", map[string]any{"workspace_id": workspaceID, "virtual_patch_id": virtualPatchID})

	virtualPatch, err := vp.Update(ctx, r.client, BuildUpdateInput(workspaceID, virtualPatchID, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating NGWAF virtual patch", err.Error())
		return
	}

	newState, err := FlattenToModel(workspaceID, virtualPatch)
	if err != nil {
		resp.Diagnostics.AddError("Error flattening NGWAF virtual patch", err.Error())
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
	virtualPatchID := state.VirtualPatchID.ValueString()
	mode := state.Mode.ValueString()

	tflog.Debug(ctx, "Disabling Fastly NGWAF workspace virtual patch during delete", map[string]any{"workspace_id": workspaceID, "virtual_patch_id": virtualPatchID})

	if _, err := vp.Update(ctx, r.client, BuildDisableInput(workspaceID, virtualPatchID, mode)); err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error disabling NGWAF virtual patch", err.Error())
	}
}

// ImportState populates workspace_id, id, and virtual_patch_id from a
// "workspace_id/virtual_patch_id" import identifier.
func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected import identifier of the form \"workspace_id/virtual_patch_id\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("virtual_patch_id"), parts[1])...)
}
