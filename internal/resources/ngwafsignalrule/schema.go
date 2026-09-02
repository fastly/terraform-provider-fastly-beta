// Package ngwafsignalrule implements the fastly_ngwaf_signal_rule resource.
package ngwafsignalrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	// RuleType is the API's rule `type` discriminator for this resource.
	RuleType = "signal"
	// ActionType is the only action a signal rule accepts; expand fills it
	// in rather than exposing it as a settable attribute.
	ActionType = "exclude_signal"
)

// Model is the fastly_ngwaf_signal_rule resource state.
type Model struct {
	ngwafrule.CommonModel
	AppliesTo   types.Set                     `tfsdk:"applies_to"`
	Description types.String                  `tfsdk:"description"`
	Action      []ngwafrule.SignalActionModel `tfsdk:"action"`
}

func resourceAttributes() map[string]schema.Attribute {
	attributes := ngwafrule.CommonAttributes()
	attributes["applies_to"] = ngwafrule.AppliesToAttribute()
	attributes["description"] = ngwafrule.DescriptionAttribute()
	return attributes
}

func resourceBlocks() map[string]schema.Block {
	blocks := ngwafrule.ConditionBlocks()
	blocks["action"] = ngwafrule.SignalActionBlock(
		"The action to take when the rule matches: exclude a signal from the matching requests. Must contain exactly 1 entry.",
		"Reference ID of the signal to exclude.",
	)
	return blocks
}
