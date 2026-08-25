package ngwafworkspacerule

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/errors"
	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"
	"github.com/fastly/terraform-provider-fastly/internal/service"

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
	_ resource.ResourceWithModifyPlan     = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_rule"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Fastly Next-Gen WAF rule scoped to a single workspace.",
		Attributes:  ResourceAttributes(),
		Blocks: map[string]schema.Block{
			"condition":          ngwafrule.ConditionBlock("Flat list of individual conditions. Each must include `field`, `operator`, and `value`."),
			"group_condition":    ngwafrule.GroupConditionBlock(),
			"multival_condition": ngwafrule.MultivalConditionBlock("List of multival conditions with nested logic. Each multival must define a `field`, `operator`, and `group_operator`, and at least one condition."),
			"action":             ActionBlock(),
			"rate_limit":         RateLimitBlock(),
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

// ValidateConfig runs the plan-time checks below; see each called
// function's own doc comment in validate.go/ngwafrule for why a given
// check exists.
func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !ngwafrule.HasAnyCondition(config.Condition, config.GroupCondition, config.MultivalCondition) {
		resp.Diagnostics.AddError(
			"Missing rule conditions",
			"A rule must define at least one 'condition', 'group_condition', or 'multival_condition'.",
		)
	}

	if total := TotalConditionCount(config.Condition, config.GroupCondition, config.MultivalCondition); total > MaxConditions {
		resp.Diagnostics.AddError(
			"Too many conditions",
			fmt.Sprintf("a rule may define at most %d combined 'condition'/'group_condition'/'multival_condition' entries, got %d.", MaxConditions, total),
		)
	}

	for _, i := range ngwafrule.EmptyGroupConditionIndexes(config.GroupCondition) {
		resp.Diagnostics.AddAttributeError(
			path.Root("group_condition").AtListIndex(i),
			"Empty group_condition",
			fmt.Sprintf("group_condition[%d] must define at least one 'condition' or 'multival_condition' block.", i),
		)
	}

	ruleType := service.StringValue(config.Type)

	if msg := InvalidDescription(ruleType, service.StringValue(config.Description)); msg != "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("description"),
			"Invalid description for rule type",
			msg,
		)
	}

	if ruleType != "request" && !config.RequestLogging.IsNull() && !config.RequestLogging.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("request_logging"),
			"Invalid request_logging for rule type",
			fmt.Sprintf("request_logging is only valid for 'request'-type rules, got a %q rule.", ruleType),
		)
	}

	for _, i := range InvalidActionTypeIndexes(ruleType, config.Action) {
		resp.Diagnostics.AddAttributeError(
			path.Root("action").AtListIndex(i).AtName("type"),
			"Invalid action type for rule type",
			fmt.Sprintf("action[%d]: %q is not a valid action type for a %q rule.", i, config.Action[i].Type.ValueString(), ruleType),
		)
	}

	if ActionCountOutOfRange(ruleType, config.Action) {
		minItems, maxItems, _ := ActionCountBounds(ruleType)
		resp.Diagnostics.AddAttributeError(
			path.Root("action"),
			"Invalid number of actions",
			fmt.Sprintf("a %q rule must define between %d and %d action(s), got %d.", ruleType, minItems, maxItems, len(config.Action)),
		)
	}

	if len(config.RateLimit) > 0 {
		for _, issue := range InvalidClientIdentifiers(config.RateLimit[0].ClientIdentifiers) {
			resp.Diagnostics.AddAttributeError(
				path.Root("rate_limit").AtListIndex(0).AtName("client_identifiers"),
				"Invalid client identifier",
				issue,
			)
		}
	}

	for i, fields := range MissingRequiredActionFields(config.Action) {
		resp.Diagnostics.AddAttributeError(
			path.Root("action").AtListIndex(i),
			"Missing required action field",
			fmt.Sprintf("action[%d] (type = %q) must set: %s.", i, config.Action[i].Type.ValueString(), strings.Join(fields, ", ")),
		)
	}

	for i, fields := range InvalidActionFieldIndexes(config.Action) {
		resp.Diagnostics.AddAttributeError(
			path.Root("action").AtListIndex(i),
			"Invalid action field for type",
			fmt.Sprintf("action[%d] (type = %q) must not set: %s.", i, config.Action[i].Type.ValueString(), strings.Join(fields, ", ")),
		)
	}
}

// templatedSignalReplaceAttributes are the top-level attributes a
// templated_signal rule cannot update in place: the API rejects an update
// to a templated_signal rule that includes actions, so a change to any of
// these forces recreation instead. workspace_id and type already force
// replacement via their own plan modifiers.
var templatedSignalReplaceAttributes = []string{
	"description", "enabled", "group_operator", "request_logging",
	"condition", "group_condition", "multival_condition", "action", "rate_limit",
}

// ModifyPlan forces recreation of a templated_signal rule whenever any
// field besides workspace_id/type changes (see
// templatedSignalReplaceAttributes).
func (r *Resource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state, plan Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if service.StringValue(state.Type) != "templated_signal" || service.StringValue(plan.Type) != "templated_signal" {
		return
	}

	changed := map[string]bool{
		"description":        !plan.Description.Equal(state.Description),
		"enabled":            !plan.Enabled.Equal(state.Enabled),
		"group_operator":     !plan.GroupOperator.Equal(state.GroupOperator),
		"request_logging":    !plan.RequestLogging.Equal(state.RequestLogging),
		"condition":          !reflect.DeepEqual(plan.Condition, state.Condition),
		"group_condition":    !reflect.DeepEqual(plan.GroupCondition, state.GroupCondition),
		"multival_condition": !reflect.DeepEqual(plan.MultivalCondition, state.MultivalCondition),
		"action":             !reflect.DeepEqual(plan.Action, state.Action),
		"rate_limit":         !reflect.DeepEqual(plan.RateLimit, state.RateLimit),
	}

	for _, name := range templatedSignalReplaceAttributes {
		if changed[name] {
			resp.RequiresReplace = append(resp.RequiresReplace, path.Root(name))
		}
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := BuildCreateInput(plan)

	tflog.Debug(ctx, "Creating Fastly NGWAF workspace rule", map[string]any{"workspace_id": service.StringValue(plan.WorkspaceID)})

	rule, err := rules.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating NGWAF workspace rule", err.Error())
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
	tflog.Debug(ctx, "Reading Fastly NGWAF workspace rule", map[string]any{"id": ruleID, "workspace_id": workspaceID})

	rule, err := rules.Get(ctx, r.client, &rules.GetInput{
		RuleID: &ruleID,
		Scope:  buildScope(workspaceID),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "NGWAF workspace rule not found, removing from state", map[string]any{"id": ruleID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NGWAF workspace rule", err.Error())
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
	input := BuildUpdateInput(ruleID, plan)

	tflog.Debug(ctx, "Updating Fastly NGWAF workspace rule", map[string]any{"id": ruleID})

	rule, err := rules.Update(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating NGWAF workspace rule", err.Error())
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
	tflog.Debug(ctx, "Deleting Fastly NGWAF workspace rule", map[string]any{"id": ruleID, "workspace_id": workspaceID})

	err := rules.Delete(ctx, r.client, &rules.DeleteInput{
		RuleID: &ruleID,
		Scope:  buildScope(workspaceID),
	})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting NGWAF workspace rule", err.Error())
	}
}

// ImportState accepts "workspace_id/rule_id", matching the legacy
// provider's import format for this resource.
func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected import identifier of the form \"workspace_id/rule_id\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
