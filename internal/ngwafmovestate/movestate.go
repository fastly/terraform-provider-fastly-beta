// Package ngwafmovestate moves legacy SDKv2 NGWAF resource state to the
// dedicated Plugin Framework resource types through Terraform moved blocks.
//
// A mover copies only what Read needs to locate the remote object: its ID
// and, for workspace-scoped objects, the workspace ID. Every other attribute
// is rebuilt from the API on the first refresh, so the per-type shape
// differences between legacy and target schemas never pass through a mover.
package ngwafmovestate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	legacyProviderAddress = "registry.terraform.io/fastly/fastly"
	legacySchemaVersion   = int64(0)
)

// Source is one legacy SDKv2 NGWAF resource type. Targets maps each legacy
// `type` value to its dedicated target type; a nil Targets is a plain rename.
type Source struct {
	TypeName  string
	Workspace bool
	Target    string
	Targets   map[string]string
	// Extra lists further string attributes Read needs, copied unchanged.
	Extra  []string
	schema func() schema.Schema
}

var (
	AccountList = Source{
		TypeName: "fastly_ngwaf_account_list",
		Targets: map[string]string{
			"country":  "fastly_ngwaf_country_list",
			"ip":       "fastly_ngwaf_ip_list",
			"signal":   "fastly_ngwaf_signal_list",
			"string":   "fastly_ngwaf_string_list",
			"wildcard": "fastly_ngwaf_wildcard_list",
		},
		schema: legacyAccountListSchema,
	}
	WorkspaceList = Source{
		TypeName:  "fastly_ngwaf_workspace_list",
		Workspace: true,
		Targets: map[string]string{
			"country":  "fastly_ngwaf_workspace_country_list",
			"ip":       "fastly_ngwaf_workspace_ip_list",
			"signal":   "fastly_ngwaf_workspace_signal_list",
			"string":   "fastly_ngwaf_workspace_string_list",
			"wildcard": "fastly_ngwaf_workspace_wildcard_list",
		},
		schema: legacyWorkspaceListSchema,
	}
	AccountRule = Source{
		TypeName: "fastly_ngwaf_account_rule",
		Targets: map[string]string{
			"request": "fastly_ngwaf_request_rule",
			"signal":  "fastly_ngwaf_signal_rule",
		},
		schema: legacyAccountRuleSchema,
	}
	WorkspaceRule = Source{
		TypeName:  "fastly_ngwaf_workspace_rule",
		Workspace: true,
		Targets: map[string]string{
			"rate_limit":       "fastly_ngwaf_workspace_rate_limit_rule",
			"request":          "fastly_ngwaf_workspace_request_rule",
			"signal":           "fastly_ngwaf_workspace_signal_rule",
			"templated_signal": "fastly_ngwaf_workspace_templated_signal_rule",
		},
		schema: legacyWorkspaceRuleSchema,
	}
	AccountSignal = Source{
		TypeName: "fastly_ngwaf_account_signal",
		Target:   "fastly_ngwaf_signal",
		schema:   legacyAccountSignalSchema,
	}
	Redaction = Source{
		TypeName:  "fastly_ngwaf_redaction",
		Workspace: true,
		Target:    "fastly_ngwaf_workspace_redaction",
		schema:    legacyRedactionSchema,
	}
	Thresholds = Source{
		TypeName:  "fastly_ngwaf_thresholds",
		Workspace: true,
		Target:    "fastly_ngwaf_workspace_threshold",
		schema:    legacyThresholdsSchema,
	}
	VirtualPatches = Source{
		TypeName:  "fastly_ngwaf_virtual_patches",
		Workspace: true,
		Target:    "fastly_ngwaf_workspace_virtual_patch",
		Extra:     []string{"virtual_patch_id"},
		schema:    legacyVirtualPatchesSchema,
	}
)

// AlertSource returns the legacy source for one alert integration type
// suffix, such as "slack" or "microsoft_teams".
func AlertSource(suffix string) (Source, bool) {
	source, ok := alertSources[suffix]
	return source, ok
}

var alertSources = map[string]Source{
	"datadog":         alert("datadog", legacyAlertDatadogIntegrationSchema),
	"jira":            alert("jira", legacyAlertJiraIntegrationSchema),
	"mailing_list":    alert("mailing_list", legacyAlertMailingListIntegrationSchema),
	"microsoft_teams": alert("microsoft_teams", legacyAlertMicrosoftTeamsIntegrationSchema),
	"opsgenie":        alert("opsgenie", legacyAlertOpsgenieIntegrationSchema),
	"pagerduty":       alert("pagerduty", legacyAlertPagerdutyIntegrationSchema),
	"slack":           alert("slack", legacyAlertSlackIntegrationSchema),
	"webhook":         alert("webhook", legacyAlertWebhookIntegrationSchema),
}

func alert(suffix string, legacySchema func() schema.Schema) Source {
	return Source{
		TypeName:  "fastly_ngwaf_alert_" + suffix + "_integration",
		Workspace: true,
		Target:    "fastly_ngwaf_workspace_alert_" + suffix + "_integration",
		schema:    legacySchema,
	}
}

// Sources returns every legacy NGWAF source, for tests that must cover all.
func Sources() []Source {
	sources := []Source{AccountList, WorkspaceList, AccountRule, WorkspaceRule, AccountSignal, Redaction, Thresholds, VirtualPatches}
	for _, suffix := range sortedKeys(alertSources) {
		sources = append(sources, alertSources[suffix])
	}
	return sources
}

// LegacySchema returns the decode-only schema for source.
func (s Source) LegacySchema() schema.Schema {
	return s.schema()
}

// TargetTypes returns every target type name the source can move to.
func (s Source) TargetTypes() []string {
	if s.Targets == nil {
		return []string{s.Target}
	}
	targets := make([]string, 0, len(s.Targets))
	for _, kind := range sortedKeys(s.Targets) {
		targets = append(targets, s.Targets[kind])
	}
	return targets
}

// Mover returns a state mover from source to the target type name.
func Mover(source Source, target string) resource.StateMover {
	legacySchema := source.schema()
	return resource.StateMover{
		SourceSchema: &legacySchema,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
			// Other sources are left to Terraform, which reports an
			// unsupported move when no mover accepts them.
			if req.SourceProviderAddress != legacyProviderAddress || req.SourceTypeName != source.TypeName {
				return
			}
			move(ctx, source, target, req, resp)
		},
	}
}

func move(ctx context.Context, source Source, target string, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if req.SourceSchemaVersion != legacySchemaVersion {
		resp.Diagnostics.AddError(
			"Unsupported legacy Fastly NGWAF state",
			fmt.Sprintf("%s can only move state from %s schema version %d; received schema version %d.", target, source.TypeName, legacySchemaVersion, req.SourceSchemaVersion),
		)
		return
	}
	if req.SourceState == nil {
		resp.Diagnostics.AddError(
			"Unable to decode legacy Fastly NGWAF state",
			fmt.Sprintf("The %s state cannot be decoded as schema version %d. Make sure it was written by fastly/fastly 9.1.0 or later (before 20.0.0), run terraform apply -refresh-only with that provider, then retry the move.", source.TypeName, legacySchemaVersion),
		)
		return
	}

	checkTarget(ctx, source, target, req.SourceState, &resp.Diagnostics)
	id := requiredString(ctx, source, req.SourceState, "id", &resp.Diagnostics)
	workspaceID := types.StringNull()
	if source.Workspace {
		workspaceID = requiredString(ctx, source, req.SourceState, "workspace_id", &resp.Diagnostics)
	}
	extra := make(map[string]types.String, len(source.Extra))
	for _, name := range source.Extra {
		extra[name] = requiredString(ctx, source, req.SourceState, name, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("id"), id)...)
	if source.Workspace {
		resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root("workspace_id"), workspaceID)...)
	}
	for _, name := range source.Extra {
		resp.Diagnostics.Append(resp.TargetState.SetAttribute(ctx, path.Root(name), extra[name])...)
	}
	if source.Workspace {
		resourceidentity.Set(ctx, resp.TargetIdentity, &resp.Diagnostics, resourceidentity.NGWAFWorkspaceObject(workspaceID, id))
	} else {
		resourceidentity.Set(ctx, resp.TargetIdentity, &resp.Diagnostics, resourceidentity.NGWAFAccountObject(id))
	}
}

// checkTarget rejects state whose legacy `type` belongs to a sibling target.
// Moving it anyway would hand the target a remote object it refuses to read.
func checkTarget(ctx context.Context, source Source, target string, state *tfsdk.State, diags *diag.Diagnostics) {
	if source.Targets == nil {
		if target != source.Target {
			diags.AddError("Unsupported legacy Fastly NGWAF move", fmt.Sprintf("%s state cannot move to %s; move it to %s.", source.TypeName, target, source.Target))
		}
		return
	}

	kind := requiredString(ctx, source, state, "type", diags)
	if diags.HasError() {
		return
	}
	want, known := source.Targets[kind.ValueString()]
	switch {
	case !known:
		diags.AddError(
			"Unsupported legacy Fastly NGWAF type",
			fmt.Sprintf("%s state has type %q, which has no dedicated resource type. Supported types: %s.", source.TypeName, kind.ValueString(), strings.Join(sortedKeys(source.Targets), ", ")),
		)
	case want != target:
		diags.AddError(
			"Legacy Fastly NGWAF type does not match the target resource",
			fmt.Sprintf("%s state has type %q and must move to %s, not %s. Update the moved block and resource type to %s.", source.TypeName, kind.ValueString(), want, target, want),
		)
	}
}

func requiredString(ctx context.Context, source Source, state *tfsdk.State, name string, diags *diag.Diagnostics) types.String {
	var value types.String
	d := state.GetAttribute(ctx, path.Root(name), &value)
	diags.Append(d...)
	if d.HasError() {
		return types.StringNull()
	}
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		diags.AddError(
			"Incomplete legacy Fastly NGWAF state",
			fmt.Sprintf("%s state has no %s. Run terraform apply -refresh-only with the legacy provider so the state is complete, then retry the move.", source.TypeName, name),
		)
		return types.StringNull()
	}
	return value
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
