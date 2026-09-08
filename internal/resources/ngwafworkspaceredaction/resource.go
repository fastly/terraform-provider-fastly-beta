package ngwafworkspaceredaction

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/redactions"
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
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_redaction"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Fastly Next-Gen WAF field redaction scoped to a single workspace.",
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

	tflog.Debug(ctx, "Creating Fastly NGWAF workspace redaction", map[string]any{
		"workspace_id": workspaceID,
		"field":        plan.Field.ValueString(),
	})

	redaction, err := redactions.Create(ctx, r.client, BuildCreateInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating NGWAF workspace redaction", err.Error())
		return
	}

	state := FlattenToModel(workspaceID, redaction)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	redactionID := state.ID.ValueString()

	tflog.Debug(ctx, "Reading Fastly NGWAF workspace redaction", map[string]any{
		"workspace_id": workspaceID,
		"id":           redactionID,
	})

	redaction, err := redactions.Get(ctx, r.client, BuildGetInput(workspaceID, redactionID))
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF workspace redaction not found, removing from state", map[string]any{
				"workspace_id": workspaceID,
				"id":           redactionID,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF workspace redaction", err.Error())
		return
	}

	newState := FlattenToModel(workspaceID, redaction)

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

	workspaceID := plan.WorkspaceID.ValueString()
	redactionID := state.ID.ValueString()

	tflog.Debug(ctx, "Updating Fastly NGWAF workspace redaction", map[string]any{
		"workspace_id": workspaceID,
		"id":           redactionID,
	})

	redaction, err := redactions.Update(ctx, r.client, BuildUpdateInput(redactionID, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating NGWAF workspace redaction", err.Error())
		return
	}

	newState := FlattenToModel(workspaceID, redaction)

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	redactionID := state.ID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly NGWAF workspace redaction", map[string]any{
		"workspace_id": workspaceID,
		"id":           redactionID,
	})

	err := redactions.Delete(ctx, r.client, BuildDeleteInput(workspaceID, redactionID))
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting NGWAF workspace redaction", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	workspaceID, redactionID, err := ParseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), workspaceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), redactionID)...)
}
