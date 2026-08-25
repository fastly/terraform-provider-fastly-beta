package ngwafrule

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var (
	conditionOperators = []string{
		"equals", "does_not_equal", "contains", "does_not_contain", "like", "not_like",
		"in_list", "not_in_list", "matches", "does_not_match", "greater_equal", "lesser_equal",
	}
	groupOperators = []string{"any", "all"}
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
	conditionOperatorDescriptor         = "One of `equals`, `does_not_equal`, `contains`, `does_not_contain`, `like`, `not_like`, `in_list`, `not_in_list`, `matches`, `does_not_match`, `greater_equal`, or `lesser_equal`."
	conditionFieldDescriptor            = "One of `agent_name`, `country`, `domain`, `ip`, `ip_remote`, `ja3_fingerprint`, `ja4_fingerprint`, `key_name`, `method`, `parameter_name`, `parameter_value`, `path`, `protocol_version`, `response_code`, `scheme`, or `user_agent`."
	multivalSubconditionFieldDescriptor = "One of `name`, `value`, `value_string`, `value_int`, `value_ip`, `signal_id`, `parameter_name`, or `parameter_value`."
)

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
					Description: "Field to inspect. One of `post_parameter`, `query_parameter`, `request_cookie`, `request_header`, `response_header`, or `signal`.",
					Validators: []validator.String{
						stringvalidator.OneOf(multivalConditionFields...),
					},
				},
				"operator": schema.StringAttribute{
					Required:    true,
					Description: "Whether the nested conditions check for existence or non-existence of matching field values. One of `exists` or `does_not_exist`.",
					Validators: []validator.String{
						stringvalidator.OneOf(multivalConditionOperators...),
					},
				},
				"group_operator": schema.StringAttribute{
					Required:    true,
					Description: "Logical operator used to evaluate the nested conditions. One of `any` or `all`.",
					Validators: []validator.String{
						stringvalidator.OneOf(groupOperators...),
					},
				},
			},
			Blocks: map[string]schema.Block{
				"condition": MultivalSubconditionBlock(
					"Nested conditions evaluated against the multival field. At least one is required.",
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
					Description: "Logical operator for the group. One of `any` or `all`.",
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
