// Package ngwafrule holds the condition / group_condition / multival_condition
// builders shared by Next-Gen WAF rule resources.
package ngwafrule

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// CommonModel is the slice of state every workspace-scoped rule resource
// holds, whatever its rule type. Each resource embeds it and adds the
// attributes its own type accepts.
type CommonModel struct {
	ID                types.String             `tfsdk:"id"`
	WorkspaceID       types.String             `tfsdk:"workspace_id"`
	Enabled           types.Bool               `tfsdk:"enabled"`
	GroupOperator     types.String             `tfsdk:"group_operator"`
	Condition         []ConditionModel         `tfsdk:"condition"`
	GroupCondition    []GroupConditionModel    `tfsdk:"group_condition"`
	MultivalCondition []MultivalConditionModel `tfsdk:"multival_condition"`
}

// ConditionModel is a single flat match condition. It is the leaf shape
// shared by the rule's top-level condition list, each group_condition's
// nested condition list, and each multival_condition's nested condition
// list.
type ConditionModel struct {
	Field    types.String `tfsdk:"field"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

// MultivalConditionModel groups conditions evaluated against a multi-value
// field (e.g. all query parameters), used both as a rule's top-level
// multival_condition list and nested inside a group_condition.
type MultivalConditionModel struct {
	Field         types.String     `tfsdk:"field"`
	Operator      types.String     `tfsdk:"operator"`
	GroupOperator types.String     `tfsdk:"group_operator"`
	Condition     []ConditionModel `tfsdk:"condition"`
}

// GroupConditionModel groups single and/or multival conditions under one
// logical operator.
type GroupConditionModel struct {
	GroupOperator     types.String             `tfsdk:"group_operator"`
	Condition         []ConditionModel         `tfsdk:"condition"`
	MultivalCondition []MultivalConditionModel `tfsdk:"multival_condition"`
}

// ActionModel is one action to take when a rule's conditions match. It
// pairs with ActionBlock, for rule types whose action set spans more than
// one action shape.
type ActionModel struct {
	Type             types.String `tfsdk:"type"`
	Signal           types.String `tfsdk:"signal"`
	AllowInteractive types.Bool   `tfsdk:"allow_interactive"`
	DeceptionType    types.String `tfsdk:"deception_type"`
	RedirectURL      types.String `tfsdk:"redirect_url"`
	ResponseCode     types.Int64  `tfsdk:"response_code"`
}

// SignalActionModel is the action shape for rule types that accept exactly
// one action taking only a signal. It pairs with SignalActionBlock, which
// omits `type` because the resource supplies the single valid value.
type SignalActionModel struct {
	Signal types.String `tfsdk:"signal"`
}
