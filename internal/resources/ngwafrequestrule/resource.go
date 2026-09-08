package ngwafrequestrule

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/path"
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
	resp.TypeName = req.ProviderTypeName + "_ngwaf_request_rule"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Fastly Next-Gen WAF `request` rule defined at account scope: inspects incoming requests and allows, blocks, or tags them in every workspace named in `applies_to`. The action block accepts `allow`, `block`, and `add_signal`. A rule must define between 1 and 10 combined `condition`, `group_condition`, and `multival_condition` entries.",
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
	ngwafrule.ValidateAccountActions(config.Action, &resp.Diagnostics)
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

	tflog.Debug(ctx, "Creating Fastly NGWAF account request rule", map[string]any{"applies_to": input.Scope.AppliesTo})

	rule, err := rules.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating NGWAF account request rule", err.Error())
		return
	}

	state, diags := FlattenToModel(ctx, rule)
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

	ruleID := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly NGWAF account request rule", map[string]any{"id": ruleID})

	rule, err := rules.Get(ctx, r.client, &rules.GetInput{
		RuleID: &ruleID,
		Scope:  ngwafrule.AccountScopeByID(),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF account request rule not found, removing from state", map[string]any{"id": ruleID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF account request rule", err.Error())
		return
	}

	ngwafrule.CheckAccountScope(rule, &resp.Diagnostics)
	ngwafrule.CheckType(RuleType, rule, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := FlattenToModel(ctx, rule)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

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
	tflog.Debug(ctx, "Updating Fastly NGWAF account request rule", map[string]any{"id": ruleID})

	input, diags := BuildUpdateInput(ctx, ruleID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := rules.Update(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating NGWAF account request rule", err.Error())
		return
	}

	newState, diags := FlattenToModel(ctx, rule)
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

	ruleID := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly NGWAF account request rule", map[string]any{"id": ruleID})

	err := rules.Delete(ctx, r.client, &rules.DeleteInput{
		RuleID: &ruleID,
		Scope:  ngwafrule.AccountScopeByID(),
	})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting NGWAF account request rule", err.Error())
	}
}

// ImportState accepts a bare rule ID: the account rules endpoint addresses a
// rule by ID alone, and Read repopulates applies_to from the API.
func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
