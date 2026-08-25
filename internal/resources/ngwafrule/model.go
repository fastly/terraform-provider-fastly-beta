// Package ngwafrule holds the condition / group_condition / multival_condition
// builders shared by Next-Gen WAF rule resources.
package ngwafrule

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
