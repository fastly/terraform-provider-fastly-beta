package ngwafalertintegration

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithConfigure   = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

type Resource struct {
	client *fastly.Client
	def    Definition
}

func NewWorkspaceResource(def Definition) resource.Resource {
	return &Resource{def: def}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_alert_" + r.def.TypeSuffix + "_integration"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: r.def.Description,
		Attributes:  Attributes(r.def),
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
	plan, diags := ModelFromPlan(ctx, req.Plan, r.def)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly NGWAF workspace alert integration", map[string]any{
		"workspace_id": plan.WorkspaceID.ValueString(),
		"type":         r.def.Type,
	})

	alert, err := r.def.Operations.Create(ctx, r.client, plan)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error creating NGWAF workspace %s alert integration", r.def.Type), err.Error())
		return
	}

	state, err := FlattenToModel(r.def, plan.WorkspaceID.ValueString(), alert)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error reading NGWAF workspace %s alert integration", r.def.Type), err.Error())
		return
	}

	resp.Diagnostics.Append(SetModelState(ctx, &resp.State, r.def, state)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	state, diags := ModelFromState(ctx, req.State, r.def)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	alertID := state.ID.ValueString()

	tflog.Debug(ctx, "Reading Fastly NGWAF workspace alert integration", map[string]any{
		"workspace_id": workspaceID,
		"id":           alertID,
		"type":         r.def.Type,
	})

	alert, err := r.def.Operations.Get(ctx, r.client, workspaceID, alertID)
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF workspace alert integration not found, removing from state", map[string]any{
				"workspace_id": workspaceID,
				"id":           alertID,
				"type":         r.def.Type,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("Error reading NGWAF workspace %s alert integration", r.def.Type), err.Error())
		return
	}

	newState, err := FlattenToModel(r.def, workspaceID, alert)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error reading NGWAF workspace %s alert integration", r.def.Type), err.Error())
		return
	}

	resp.Diagnostics.Append(SetModelState(ctx, &resp.State, r.def, newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	plan, planDiags := ModelFromPlan(ctx, req.Plan, r.def)
	resp.Diagnostics.Append(planDiags...)

	state, stateDiags := ModelFromState(ctx, req.State, r.def)
	resp.Diagnostics.Append(stateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	alertID := state.ID.ValueString()

	tflog.Debug(ctx, "Updating Fastly NGWAF workspace alert integration", map[string]any{
		"workspace_id": plan.WorkspaceID.ValueString(),
		"id":           alertID,
		"type":         r.def.Type,
	})

	alert, err := r.def.Operations.Update(ctx, r.client, alertID, plan)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error updating NGWAF workspace %s alert integration", r.def.Type), err.Error())
		return
	}

	newState, err := FlattenToModel(r.def, plan.WorkspaceID.ValueString(), alert)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error reading NGWAF workspace %s alert integration", r.def.Type), err.Error())
		return
	}

	resp.Diagnostics.Append(SetModelState(ctx, &resp.State, r.def, newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	state, diags := ModelFromState(ctx, req.State, r.def)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	alertID := state.ID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly NGWAF workspace alert integration", map[string]any{
		"workspace_id": workspaceID,
		"id":           alertID,
		"type":         r.def.Type,
	})

	err := r.def.Operations.Delete(ctx, r.client, workspaceID, alertID)
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError(fmt.Sprintf("Error deleting NGWAF workspace %s alert integration", r.def.Type), err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	ImportState(ctx, req, resp)
}
