package routingconfigauto

import (
	"context"
	"fmt"
	"sort"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/reconcile"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v18/fastly"
	"github.com/fastly/go-fastly/v18/fastly/domainmanagement/v1/routingconfigs"
	"github.com/fastly/go-fastly/v18/fastly/domainmanagement/v1/routingconfigs/paths"
	"github.com/fastly/go-fastly/v18/fastly/domainmanagement/v1/routingconfigs/paths/rules"
)

var (
	_ resource.Resource                   = &Resource{}
	_ resource.ResourceWithConfigure      = &Resource{}
	_ resource.ResourceWithValidateConfig = &Resource{}
	_ resource.ResourceWithImportState    = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_routing_config_auto"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Routing configs let you route requests to different services (or other destinations) based on URL path, using conditional rules. This resource manages a routing config's `path` and `rule` configuration using automatic versioning: every apply reconciles the routing config's draft paths and rules against this resource's configuration and, if anything changed, activates the result as the new active version. The underlying draft/version workflow exposed by the API is never surfaced to Terraform. A routing config isn't useful on its own; it must be linked to one or more domains via `fastly_routing_config_domain_link` before it will route traffic.",
		Attributes:  ResourceAttributes(),
		Blocks: map[string]schema.Block{
			"path": PathBlock(),
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

// ValidateConfig requires at least one `path` and one `rule` per path, including a default
// (condition-less) rule (list blocks can't be marked Required), requires `key` on header conditions, and
// rejects duplicate `path` blocks: Create/Update key paths by their `path`
// string and would otherwise silently collide.
func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(config.Path) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("path"),
			"Missing Routing Config Path",
			"at least one `path` block is required",
		)
	}

	seen := make(map[string]bool, len(config.Path))
	for i, pm := range config.Path {
		validateRules(i, pm, &resp.Diagnostics)

		if pm.Path.IsUnknown() {
			continue
		}
		pathStr := pm.Path.ValueString()
		if seen[pathStr] {
			resp.Diagnostics.AddAttributeError(
				path.Root("path").AtListIndex(i).AtName("path"),
				"Duplicate Routing Config Path",
				fmt.Sprintf("duplicate path %q: each `path` block must have a unique `path` value", pathStr),
			)
			continue
		}
		seen[pathStr] = true
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	tflog.Debug(ctx, "Creating Fastly Routing Config", map[string]any{"name": name})

	created, err := routingconfigs.Create(ctx, r.client, &routingconfigs.CreateInput{Name: &name})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Routing Config", err.Error())
		return
	}
	routingConfigID := created.RoutingConfigID

	// Save state as soon as the ID exists, so a later failure here still lets
	// a destroy clean up the routing config instead of leaking it.
	var state Model
	applyTopLevel(&state, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pathList, err := createPaths(ctx, r.client, routingConfigID, plan.Path)
	state.Path = pathList
	if err != nil {
		resp.Diagnostics.AddError("Error creating Routing Config paths", err.Error())
	}
	// Persisted even on failure: the framework taints a resource created with
	// an error, so the next apply force-deletes this partial routing config.
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	activated, err := routingconfigs.Activate(ctx, r.client, &routingconfigs.ActivateInput{RoutingConfigID: &routingConfigID})
	if err != nil {
		resp.Diagnostics.AddError("Error activating Routing Config", err.Error())
		return
	}

	applyTopLevel(&state, activated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// createPaths builds the resulting `path` state from the create responses,
// returning what was created so far alongside any error.
func createPaths(ctx context.Context, client *fastly.Client, routingConfigID string, desired []PathModel) ([]PathModel, error) {
	pathList := make([]PathModel, 0, len(desired))
	for _, pm := range desired {
		pathStr := pm.Path.ValueString()

		createdPath, err := paths.Create(ctx, client, &paths.CreateInput{
			RoutingConfigID: &routingConfigID,
			Path:            &pathStr,
		})
		if err != nil {
			return pathList, fmt.Errorf("failed to create routing config path %q: %w", pathStr, err)
		}

		ruleList, err := createRules(ctx, client, routingConfigID, createdPath.PathID, pm.Rule)
		if err != nil {
			return pathList, fmt.Errorf("failed to create rules for routing config path %q: %w", pathStr, err)
		}

		pathList = append(pathList, PathModel{
			Path:   pm.Path,
			PathID: types.StringValue(createdPath.PathID),
			Rule:   ruleList,
		})
	}
	return pathList, nil
}

// createRules builds the resulting `rule` state from the create responses.
func createRules(ctx context.Context, client *fastly.Client, routingConfigID, pathID string, desired []RuleModel) ([]RuleModel, error) {
	ruleList := make([]RuleModel, 0, len(desired))
	for _, rm := range desired {
		action, conditions := expandRule(rm)

		created, err := rules.Create(ctx, client, &rules.CreateInput{
			RoutingConfigID: &routingConfigID,
			PathID:          &pathID,
			Action:          &action,
			Conditions:      conditions,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create rule: %w", err)
		}
		ruleList = append(ruleList, flattenRule(*created))
	}
	return ruleList, nil
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly Routing Config", map[string]any{"id": id})

	data, err := routingconfigs.Get(ctx, r.client, &routingconfigs.GetInput{RoutingConfigID: &id})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Routing Config not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Routing Config", err.Error())
		return
	}

	pathList, err := listPaths(ctx, r.client, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Routing Config", err.Error())
		return
	}

	var newState Model
	applyTopLevel(&newState, data)
	newState.Path = matchPathOrder(pathList, state.Path)

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// listPaths lists every path (and each path's rules) on the routing config.
func listPaths(ctx context.Context, client *fastly.Client, routingConfigID string) ([]PathModel, error) {
	actualPaths, err := paths.List(ctx, client, &paths.ListInput{RoutingConfigID: &routingConfigID})
	if err != nil {
		return nil, fmt.Errorf("failed to list routing config paths: %w", err)
	}

	pathList := make([]PathModel, 0, len(actualPaths))
	for _, p := range actualPaths {
		actualRules, err := rules.List(ctx, client, &rules.ListInput{RoutingConfigID: &routingConfigID, PathID: &p.PathID})
		if err != nil {
			return nil, fmt.Errorf("failed to list rules for routing config path %q: %w", p.Path, err)
		}
		ruleList := make([]RuleModel, 0, len(actualRules))
		for _, rl := range actualRules {
			ruleList = append(ruleList, flattenRule(rl))
		}
		pathList = append(pathList, PathModel{
			Path:   types.StringValue(p.Path),
			PathID: types.StringValue(p.PathID),
			Rule:   ruleList,
		})
	}
	return pathList, nil
}

// matchPathOrder reorders freshly-listed paths to match prior's order, so API
// list-order churn doesn't show up as plan noise. Rules keep the API's order,
// since it is their evaluation order.
func matchPathOrder(actual, prior []PathModel) []PathModel {
	return reconcile.MatchOrder(actual, prior, func(p PathModel) string { return p.Path.ValueString() })
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	var state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Updating Fastly Routing Config", map[string]any{"id": id})

	// Reconcile against the live draft, not state: a failed earlier Update may
	// have left paths or rules in the draft that state doesn't know about.
	livePaths, err := listPaths(ctx, r.client, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Routing Config", err.Error())
		return
	}

	oldByPath := make(map[string]PathModel, len(livePaths))
	for _, pm := range livePaths {
		oldByPath[pm.Path.ValueString()] = pm
	}

	desiredPaths := make(map[string]bool, len(plan.Path))
	for _, pm := range plan.Path {
		desiredPaths[pm.Path.ValueString()] = true
	}

	// A draft that differs from state is unactivated work from a failed
	// Update, so it needs activating even if it already matches the plan.
	changed, err := pathsDiffer(livePaths, state.Path)
	if err != nil {
		resp.Diagnostics.AddError("Error comparing Routing Config paths", err.Error())
		return
	}

	for pathStr, pm := range oldByPath {
		if desiredPaths[pathStr] {
			continue
		}
		pathID := pm.PathID.ValueString()
		if err := paths.Delete(ctx, r.client, &paths.DeleteInput{RoutingConfigID: &id, PathID: &pathID}); err != nil && !errors.IsNotFound(err) {
			resp.Diagnostics.AddError("Error deleting Routing Config path", fmt.Sprintf("failed to delete routing config path %q: %s", pathStr, err))
			return
		}
		changed = true
	}

	pathList := make([]PathModel, 0, len(plan.Path))
	for _, pm := range plan.Path {
		pathStr := pm.Path.ValueString()

		if old, ok := oldByPath[pathStr]; ok {
			pathID := old.PathID.ValueString()
			ruleList, rulesChanged, err := reconcileRules(ctx, r.client, id, pathID, old.Rule, pm.Rule)
			if err != nil {
				resp.Diagnostics.AddError("Error reconciling Routing Config rules", fmt.Sprintf("failed to reconcile rules for routing config path %q: %s", pathStr, err))
				return
			}
			if rulesChanged {
				changed = true
			}
			pathList = append(pathList, PathModel{
				Path:   pm.Path,
				PathID: old.PathID,
				Rule:   ruleList,
			})
			continue
		}

		createdPath, err := paths.Create(ctx, r.client, &paths.CreateInput{RoutingConfigID: &id, Path: &pathStr})
		if err != nil {
			resp.Diagnostics.AddError("Error creating Routing Config path", fmt.Sprintf("failed to create routing config path %q: %s", pathStr, err))
			return
		}
		ruleList, err := createRules(ctx, r.client, id, createdPath.PathID, pm.Rule)
		if err != nil {
			resp.Diagnostics.AddError("Error creating Routing Config rules", fmt.Sprintf("failed to create rules for routing config path %q: %s", pathStr, err))
			return
		}
		pathList = append(pathList, PathModel{
			Path:   pm.Path,
			PathID: types.StringValue(createdPath.PathID),
			Rule:   ruleList,
		})
		changed = true
	}

	var newState Model
	if changed {
		activated, err := routingconfigs.Activate(ctx, r.client, &routingconfigs.ActivateInput{RoutingConfigID: &id})
		if err != nil {
			resp.Diagnostics.AddError("Error activating Routing Config", err.Error())
			return
		}
		applyTopLevel(&newState, activated)
	} else {
		data, err := routingconfigs.Get(ctx, r.client, &routingconfigs.GetInput{RoutingConfigID: &id})
		if err != nil {
			resp.Diagnostics.AddError("Error reading Routing Config", err.Error())
			return
		}
		applyTopLevel(&newState, data)
	}
	newState.Path = pathList

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// reconcileRules deletes and recreates a path's rules when they changed
// (rules have no natural key, and creation order sets evaluation order),
// reporting whether anything changed.
func reconcileRules(ctx context.Context, client *fastly.Client, routingConfigID, pathID string, oldRules, desiredRules []RuleModel) ([]RuleModel, bool, error) {
	same, err := sameRules(oldRules, desiredRules)
	if err != nil {
		return nil, false, err
	}
	if same {
		return oldRules, false, nil
	}

	for _, rm := range oldRules {
		ruleID := rm.RuleID.ValueString()
		if err := rules.Delete(ctx, client, &rules.DeleteInput{RoutingConfigID: &routingConfigID, PathID: &pathID, RuleID: &ruleID}); err != nil && !errors.IsNotFound(err) {
			return nil, false, fmt.Errorf("failed to delete rule %q: %w", ruleID, err)
		}
	}

	ruleList, err := createRules(ctx, client, routingConfigID, pathID, desiredRules)
	if err != nil {
		return nil, false, err
	}

	return ruleList, true, nil
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly Routing Config", map[string]any{"id": id})

	force := true
	if err := routingconfigs.Delete(ctx, r.client, &routingconfigs.DeleteInput{RoutingConfigID: &id, Force: &force}); err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Routing Config", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID

	data, err := routingconfigs.Get(ctx, r.client, &routingconfigs.GetInput{RoutingConfigID: &id})
	if err != nil {
		resp.Diagnostics.AddError("Error importing Routing Config", err.Error())
		return
	}

	pathList, err := listPaths(ctx, r.client, id)
	if err != nil {
		resp.Diagnostics.AddError("Error importing Routing Config", err.Error())
		return
	}

	var state Model
	applyTopLevel(&state, data)
	sort.Slice(pathList, func(i, j int) bool { return pathList[i].Path.ValueString() < pathList[j].Path.ValueString() })
	state.Path = pathList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func validateRules(pathIdx int, pm PathModel, diags *diag.Diagnostics) {
	pathAttr := path.Root("path").AtListIndex(pathIdx)

	if len(pm.Rule) == 0 {
		diags.AddAttributeError(
			pathAttr.AtName("rule"),
			"Missing Routing Config Rule",
			"each `path` block requires at least one `rule` block",
		)
	}

	hasDefault := false
	for ri, rm := range pm.Rule {
		if len(rm.Condition) == 0 {
			hasDefault = true
			// The API always keeps the default rule last, so a default listed
			// earlier would be reordered on read and plan a perpetual diff.
			if ri != len(pm.Rule)-1 {
				diags.AddAttributeError(
					pathAttr.AtName("rule").AtListIndex(ri),
					"Default Routing Config Rule Must Be Last",
					"the default rule (a `rule` block with no `condition` blocks) must be the last `rule` in its `path`, and there can be only one",
				)
			}
		}
		for ci, cm := range rm.Condition {
			if cm.Type.IsUnknown() || cm.Key.IsUnknown() {
				continue
			}
			if cm.Type.ValueString() == "header" && cm.Key.ValueString() == "" {
				diags.AddAttributeError(
					pathAttr.AtName("rule").AtListIndex(ri).AtName("condition").AtListIndex(ci).AtName("key"),
					"Missing Condition Key",
					"`key` is required when the condition `type` is `header`",
				)
			}
		}
	}

	if len(pm.Rule) > 0 && !hasDefault {
		diags.AddAttributeError(
			pathAttr.AtName("rule"),
			"Missing Default Routing Config Rule",
			"each `path` requires a default rule: a `rule` block with no `condition` blocks, used when no other rule matches",
		)
	}
}
