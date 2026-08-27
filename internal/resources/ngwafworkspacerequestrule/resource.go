package ngwafworkspacerequestrule

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/errors"
	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"
	"github.com/fastly/terraform-provider-fastly/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

var (
	_ resource.Resource                   = &Resource{}
	_ resource.ResourceWithConfigure      = &Resource{}
	_ resource.ResourceWithImportState    = &Resource{}
	_ resource.ResourceWithValidateConfig = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_request_rule"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Fastly Next-Gen WAF `request` rule scoped to a single workspace: inspects incoming requests and allows, blocks, challenges, or tags them.",
		Attributes:  resourceAttributes(),
		Blocks:      resourceBlocks(),
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

	ngwafrule.ValidateConditions(config.CommonModel, &resp.Diagnostics)
	ngwafrule.ValidateActions(config.Action, &resp.Diagnostics)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly NGWAF workspace request rule", map[string]any{"workspace_id": service.StringValue(plan.WorkspaceID)})

	rule, err := rules.Create(ctx, r.client, BuildCreateInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating NGWAF workspace request rule", err.Error())
		return
	}

	state := FlattenToModel(rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleID := state.ID.ValueString()
	workspaceID := service.StringValue(state.WorkspaceID)
	tflog.Debug(ctx, "Reading Fastly NGWAF workspace request rule", map[string]any{"id": ruleID, "workspace_id": workspaceID})

	rule, err := rules.Get(ctx, r.client, &rules.GetInput{
		RuleID: &ruleID,
		Scope:  ngwafrule.WorkspaceScope(workspaceID),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF workspace request rule not found, removing from state", map[string]any{"id": ruleID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF workspace request rule", err.Error())
		return
	}

	ngwafrule.CheckType(RuleType, rule, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	newState := FlattenToModel(rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleID := state.ID.ValueString()
	tflog.Debug(ctx, "Updating Fastly NGWAF workspace request rule", map[string]any{"id": ruleID})

	rule, err := rules.Update(ctx, r.client, BuildUpdateInput(ruleID, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating NGWAF workspace request rule", err.Error())
		return
	}

	newState := FlattenToModel(rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleID := state.ID.ValueString()
	workspaceID := service.StringValue(state.WorkspaceID)
	tflog.Debug(ctx, "Deleting Fastly NGWAF workspace request rule", map[string]any{"id": ruleID, "workspace_id": workspaceID})

	err := rules.Delete(ctx, r.client, &rules.DeleteInput{
		RuleID: &ruleID,
		Scope:  ngwafrule.WorkspaceScope(workspaceID),
	})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting NGWAF workspace request rule", err.Error())
	}
}

// ImportState accepts "workspace_id/rule_id".
func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	ngwafrule.ImportState(ctx, req, resp)
}
