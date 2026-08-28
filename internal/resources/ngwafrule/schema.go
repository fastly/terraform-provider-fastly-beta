package ngwafrule

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// DefaultGroupOperator is the value the API assigns to group_operator when
// it's omitted from the request body.
const DefaultGroupOperator = "all"

var (
	conditionOperators = []string{
		"equals", "does_not_equal", "contains", "does_not_contain", "like", "not_like",
		"in_list", "not_in_list", "matches", "does_not_match", "greater_equal", "lesser_equal",
	}
	groupOperators = []string{"any", "all"}
	// deceptionTypes are the valid values for an action's deception_type,
	// required when action.type = "deception".
	deceptionTypes = []string{"invalid_login_response", "vulnerable_application_response", "ato"}
	// conditionFields are the valid `field` values for a top-level single
	// condition - used both as the rule's own `condition` list and as the
	// nested `condition` list inside a `group_condition`, which accept the
	// same values. This is a different, disjoint set from
	// multivalSubconditionFields below.
	conditionFields = []string{
		"agent_name", "country", "domain", "ip", "ip_remote", "ja3_fingerprint", "ja4_fingerprint",
		"key_name", "method", "parameter_name", "parameter_value", "path", "protocol_version",
		"response_code", "scheme", "user_agent",
	}
	multivalConditionFields    = []string{"post_parameter", "query_parameter", "request_cookie", "request_header", "response_header", "signal"}
	multivalConditionOperators = []string{"exists", "does_not_exist"}
	// multivalSubconditionFields are the valid `field` values for a
	// condition nested inside a multival_condition - a completely
	// different set from conditionFields above, so it cannot reuse
	// ConditionBlock.
	multivalSubconditionFields          = []string{"name", "value", "value_string", "value_int", "value_ip", "signal_id", "parameter_name", "parameter_value"}
	conditionOperatorDescriptor         = OneOfDescriptor(conditionOperators)
	conditionFieldDescriptor            = OneOfDescriptor(conditionFields)
	multivalSubconditionFieldDescriptor = OneOfDescriptor(multivalSubconditionFields)
)

// OneOfDescriptor renders an enum as the "One of `a`, `b`, or `c`." sentence
// used in attribute descriptions. Descriptions are the only thing
// tfplugindocs renders, so building them from the same slice that feeds the
// attribute's OneOf validator keeps the two from drifting.
func OneOfDescriptor(values []string) string {
	return fmt.Sprintf("One of %s.", orList(values))
}

func orList(values []string) string {
	return joinList(values, "or")
}

func andList(values []string) string {
	return joinList(values, "and")
}

func joinList(values []string, conjunction string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, fmt.Sprintf("`%s`", v))
	}

	switch len(quoted) {
	case 0:
		return ""
	case 1:
		return quoted[0]
	case 2:
		return quoted[0] + " " + conjunction + " " + quoted[1]
	default:
		return strings.Join(quoted[:len(quoted)-1], ", ") + ", " + conjunction + " " + quoted[len(quoted)-1]
	}
}

// ConditionBlock is the field/operator/value leaf for a top-level single
// condition. It is reused unmodified as the rule's top-level `condition`
// list and the nested `condition` list inside a `group_condition` - both
// accept the same `field` enum. It is NOT valid for the nested `condition`
// list inside a `multival_condition`, which has a disjoint `field` enum -
// see MultivalSubconditionBlock for that case.
func ConditionBlock(description string, validators ...validator.List) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		Validators:  validators,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Required:    true,
					Description: "Field to inspect. " + conditionFieldDescriptor,
					Validators: []validator.String{
						stringvalidator.OneOf(conditionFields...),
					},
				},
				"operator": schema.StringAttribute{
					Required:    true,
					Description: "Operator to apply. " + conditionOperatorDescriptor,
					Validators: []validator.String{
						stringvalidator.OneOf(conditionOperators...),
					},
				},
				"value": schema.StringAttribute{
					Required:    true,
					Description: "The value to test the field against.",
					Validators: []validator.String{
						stringvalidator.LengthAtLeast(1),
					},
				},
			},
		},
	}
}

// MultivalSubconditionBlock is the field/operator/value leaf for a
// condition nested inside a multival_condition. It has the same shape as
// ConditionBlock but a different, disjoint `field` enum, so it cannot
// reuse ConditionBlock directly.
func MultivalSubconditionBlock(description string, validators ...validator.List) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		Validators:  validators,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Required:    true,
					Description: "Field to inspect. " + multivalSubconditionFieldDescriptor,
					Validators: []validator.String{
						stringvalidator.OneOf(multivalSubconditionFields...),
					},
				},
				"operator": schema.StringAttribute{
					Required:    true,
					Description: "Operator to apply. " + conditionOperatorDescriptor,
					Validators: []validator.String{
						stringvalidator.OneOf(conditionOperators...),
					},
				},
				"value": schema.StringAttribute{
					Required:    true,
					Description: "The value to test the field against.",
					Validators: []validator.String{
						stringvalidator.LengthAtLeast(1),
					},
				},
			},
		},
	}
}

// MultivalConditionBlock groups conditions evaluated against a multi-value
// field (e.g. all query parameters, all request headers): does at least
// one (or every) post_parameter/request_header/etc. exist or not exist
// matching the nested conditions.
func MultivalConditionBlock(description string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Required:    true,
					Description: "Field to inspect. " + OneOfDescriptor(multivalConditionFields),
					Validators: []validator.String{
						stringvalidator.OneOf(multivalConditionFields...),
					},
				},
				"operator": schema.StringAttribute{
					Required:    true,
					Description: "Whether the nested conditions check for existence or non-existence of matching field values. " + OneOfDescriptor(multivalConditionOperators),
					Validators: []validator.String{
						stringvalidator.OneOf(multivalConditionOperators...),
					},
				},
				"group_operator": schema.StringAttribute{
					Required:    true,
					Description: "Logical operator used to evaluate the nested conditions. " + OneOfDescriptor(groupOperators),
					Validators: []validator.String{
						stringvalidator.OneOf(groupOperators...),
					},
				},
			},
			Blocks: map[string]schema.Block{
				"condition": MultivalSubconditionBlock(
					"Nested conditions evaluated against the multival field. At least one is required.",
					listvalidator.IsRequired(),
					listvalidator.SizeAtLeast(1),
				),
			},
		},
	}
}

// GroupConditionBlock groups single and/or multival conditions under one
// logical operator.
func GroupConditionBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "List of grouped conditions with nested logic. Each group must define a `group_operator` and at least one `condition` or `multival_condition`.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"group_operator": schema.StringAttribute{
					Required:    true,
					Description: "Logical operator for the group. " + OneOfDescriptor(groupOperators),
					Validators: []validator.String{
						stringvalidator.OneOf(groupOperators...),
					},
				},
			},
			Blocks: map[string]schema.Block{
				"condition": ConditionBlock("A list of nested conditions in this group."),
				"multival_condition": MultivalConditionBlock(
					"List of nested multival conditions in this group. Each multival must define a `field`, `operator`, and `group_operator`, and at least one condition.",
				),
			},
		},
	}
}

// ConditionBlocks are the three condition shapes every rule type accepts,
// identically. planModifiers, when given, is applied to all three.
func ConditionBlocks(planModifiers ...planmodifier.List) map[string]schema.Block {
	condition := ConditionBlock("Flat list of individual conditions. Each must include `field`, `operator`, and `value`.")
	condition.PlanModifiers = planModifiers

	group := GroupConditionBlock()
	group.PlanModifiers = planModifiers

	multival := MultivalConditionBlock("List of multival conditions with nested logic. Each multival must define a `field`, `operator`, and `group_operator`, and at least one condition.")
	multival.PlanModifiers = planModifiers

	return map[string]schema.Block{
		"condition":          condition,
		"group_condition":    group,
		"multival_condition": multival,
	}
}

// ActionBlock is the action block for rule types whose valid actions span
// more than one action shape. allowed is that rule type's action `type`
// enum; minItems/maxItems are how many actions it must declare. It pairs
// with ActionModel.
func ActionBlock(allowed []string, minItems, maxItems int) schema.ListNestedBlock {
	count := fmt.Sprintf("Must contain between %d and %d entries.", minItems, maxItems)
	if minItems == maxItems {
		count = fmt.Sprintf("Must contain exactly %d entry.", minItems)
	}

	// SizeBetween skips a null list, so a non-zero minimum needs IsRequired
	// to reject an omitted block.
	validators := []validator.List{listvalidator.SizeBetween(minItems, maxItems)}
	if minItems > 0 {
		validators = append([]validator.List{listvalidator.IsRequired()}, validators...)
	}

	return schema.ListNestedBlock{
		Description: "Actions to perform when the rule matches. " + count,
		Validators:  validators,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"type": schema.StringAttribute{
					Required:    true,
					Description: "The action type. " + OneOfDescriptor(allowed),
					Validators: []validator.String{
						stringvalidator.OneOf(allowed...),
					},
				},
				"signal": schema.StringAttribute{
					Optional:    true,
					Description: "Reference ID of the signal. " + actionFieldDescriptor("signal", allowed),
				},
				"allow_interactive": schema.BoolAttribute{
					Optional:    true,
					Description: "Specifies if interaction is allowed. " + actionFieldDescriptor("allow_interactive", allowed),
				},
				"deception_type": schema.StringAttribute{
					Optional:    true,
					Description: "Specifies the type of deception. " + actionFieldDescriptor("deception_type", allowed) + " " + OneOfDescriptor(deceptionTypes),
					Validators: []validator.String{
						stringvalidator.OneOf(deceptionTypes...),
					},
				},
				"redirect_url": schema.StringAttribute{
					Optional:    true,
					Description: "Redirect target. " + actionFieldDescriptor("redirect_url", allowed),
				},
				"response_code": schema.Int64Attribute{
					Optional:    true,
					Description: "Response code returned to the client. " + actionFieldDescriptor("response_code", allowed),
				},
			},
		},
	}
}

// SignalActionBlock is the action block for rule types whose only valid
// action carries a signal and nothing else. `type` is omitted because the
// resource supplies the single valid value. It pairs with
// SignalActionModel.
func SignalActionBlock(description, signalDescription string, planModifiers ...planmodifier.List) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description:   description,
		PlanModifiers: planModifiers,
		Validators: []validator.List{
			listvalidator.IsRequired(),
			listvalidator.SizeBetween(1, 1),
		},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"signal": schema.StringAttribute{
					Required:    true,
					Description: signalDescription,
					Validators: []validator.String{
						stringvalidator.LengthAtLeast(1),
					},
				},
			},
		},
	}
}

// actionFieldDescriptor names which of a rule type's allowed action types
// require the given action field and which merely accept it, derived from
// the same tables the validators use.
func actionFieldDescriptor(field string, allowed []string) string {
	var required, optional []string
	for _, t := range allowed {
		if !slices.Contains(actionFields[t], field) {
			continue
		}
		if slices.Contains(actionRequiredFields[t], field) {
			required = append(required, t)
		} else {
			optional = append(optional, t)
		}
	}

	var sentences []string
	if len(required) > 0 {
		sentences = append(sentences, fmt.Sprintf("Required by %s.", andList(required)))
	}
	if len(optional) > 0 {
		sentences = append(sentences, fmt.Sprintf("Optional on %s.", andList(optional)))
	}
	if len(sentences) == 0 {
		return "Not accepted by any action type this rule allows."
	}
	return strings.Join(sentences, " ")
}

// IDAttribute is the Fastly-assigned rule ID.
func IDAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Computed:    true,
		Description: "The rule identifier generated by Fastly.",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}

// WorkspaceIDAttribute forces replacement: workspace_id is a path segment
// (/ngwaf/v1/workspaces/{workspace_id}/rules/{rule_id}), not a request body
// field, so there's no API call that moves an existing rule to a different
// workspace - an in-place update would PATCH a rule ID that doesn't exist
// under the new workspace's path and fail.
func WorkspaceIDAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required:    true,
		Description: "The ID of the workspace this rule belongs to.",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
}

// DescriptionAttribute is the rule description, for the rule types that
// require one.
func DescriptionAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required:    true,
		Description: "The description of the rule.",
		Validators: []validator.String{
			stringvalidator.LengthAtLeast(1),
		},
	}
}

// EnabledAttribute reports whether the rule is active.
func EnabledAttribute() schema.BoolAttribute {
	return schema.BoolAttribute{
		Required:    true,
		Description: "Whether the rule is currently enabled.",
	}
}

// GroupOperatorAttribute is the logical operator applied across a rule's
// top-level conditions.
func GroupOperatorAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Optional:    true,
		Computed:    true,
		Default:     stringdefault.StaticString(DefaultGroupOperator),
		Description: "Logical operator applied across the rule's top-level condition, group_condition, and multival_condition entries. " + OneOfDescriptor(groupOperators) + " Defaults to `" + DefaultGroupOperator + "`.",
		Validators: []validator.String{
			stringvalidator.OneOf(groupOperators...),
		},
	}
}

// CommonAttributes are the top-level attributes shared by every
// workspace-scoped rule resource.
func CommonAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id":             IDAttribute(),
		"workspace_id":   WorkspaceIDAttribute(),
		"enabled":        EnabledAttribute(),
		"group_operator": GroupOperatorAttribute(),
	}
}

// HasAnyCondition reports whether at least one of the rule's three
// condition shapes is present - the API requires every rule to declare at
// least one.
func HasAnyCondition(conditions []ConditionModel, groups []GroupConditionModel, multivals []MultivalConditionModel) bool {
	return len(conditions) > 0 || len(groups) > 0 || len(multivals) > 0
}

// EmptyGroupConditionIndexes returns the index of every group_condition
// entry that defines neither a nested condition nor a nested
// multival_condition - an empty group has nothing to match against.
func EmptyGroupConditionIndexes(groups []GroupConditionModel) []int {
	var empty []int
	for i, g := range groups {
		if len(g.Condition) == 0 && len(g.MultivalCondition) == 0 {
			empty = append(empty, i)
		}
	}
	return empty
}
