// Package ngwafworkspacetemplatedsignalrule implements the
// fastly_ngwaf_workspace_templated_signal_rule resource.
package ngwafworkspacetemplatedsignalrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

const (
	// RuleType is the API's rule `type` discriminator for this resource.
	RuleType = "templated_signal"
	// ActionType is the only action a templated signal rule accepts; expand
	// fills it in rather than exposing it as a settable attribute.
	ActionType = "templated_signal"
)

// Model is the fastly_ngwaf_workspace_templated_signal_rule resource state.
// There is no description: the API accepts only an empty one for this rule
// type.
type Model struct {
	ngwafrule.CommonModel
	Action []ngwafrule.SignalActionModel `tfsdk:"action"`
}

// Every attribute and block below forces replacement, because the API
// rejects an update to a templated_signal rule that carries actions and
// actions are mandatory - there is nothing this rule type can change in
// place.
func resourceAttributes() map[string]schema.Attribute {
	enabled := ngwafrule.EnabledAttribute()
	enabled.PlanModifiers = append(enabled.PlanModifiers, boolplanmodifier.RequiresReplace())

	groupOperator := ngwafrule.GroupOperatorAttribute()
	groupOperator.PlanModifiers = append(groupOperator.PlanModifiers, stringplanmodifier.RequiresReplace())

	return map[string]schema.Attribute{
		"id":             ngwafrule.IDAttribute(),
		"workspace_id":   ngwafrule.WorkspaceIDAttribute(),
		"enabled":        enabled,
		"group_operator": groupOperator,
	}
}

func resourceBlocks() map[string]schema.Block {
	blocks := ngwafrule.ConditionBlocks(listplanmodifier.RequiresReplace())
	blocks["action"] = ngwafrule.SignalActionBlock(
		"The action to take when the rule matches: add a templated signal to the matching requests. Must contain exactly 1 entry.",
		"Name of the templated signal to add, for example `LOGINATTEMPT`.",
		listplanmodifier.RequiresReplace(),
	)
	return blocks
}
