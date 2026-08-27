package ngwafrule

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

// FlattenCommon reads the state every workspace-scoped rule type holds out
// of an API rule.
func FlattenCommon(rule *rules.Rule) CommonModel {
	m := CommonModel{
		ID:      types.StringValue(rule.RuleID),
		Enabled: types.BoolValue(rule.Enabled),
	}

	if len(rule.Scope.AppliesTo) > 0 {
		m.WorkspaceID = types.StringValue(rule.Scope.AppliesTo[0])
	}
	if rule.GroupOperator != "" {
		m.GroupOperator = types.StringValue(rule.GroupOperator)
	}

	m.Condition, m.GroupCondition, m.MultivalCondition = FlattenConditions(rule.Conditions)

	return m
}

// FlattenActions reads the API's actions into the multi-shape ActionModel.
func FlattenActions(actions []rules.Action) []ActionModel {
	if len(actions) == 0 {
		return nil
	}

	result := make([]ActionModel, 0, len(actions))
	for _, a := range actions {
		action := ActionModel{
			Type: types.StringValue(a.Type),
		}
		if a.Signal != "" {
			action.Signal = types.StringValue(a.Signal)
		}
		if a.AllowInteractive != nil {
			action.AllowInteractive = types.BoolValue(*a.AllowInteractive)
		}
		if a.DeceptionType != "" {
			action.DeceptionType = types.StringValue(a.DeceptionType)
		}
		if a.RedirectURL != "" {
			action.RedirectURL = types.StringValue(a.RedirectURL)
		}
		if a.ResponseCode != 0 {
			action.ResponseCode = types.Int64Value(int64(a.ResponseCode))
		}
		result = append(result, action)
	}
	return result
}

// FlattenSignalActions reads the API's actions into SignalActionModel,
// dropping the action type the resource already fixes.
func FlattenSignalActions(actions []rules.Action) []SignalActionModel {
	if len(actions) == 0 {
		return nil
	}

	result := make([]SignalActionModel, 0, len(actions))
	for _, a := range actions {
		result = append(result, SignalActionModel{
			Signal: types.StringValue(a.Signal),
		})
	}
	return result
}

// FlattenConditions splits the API's single polymorphic condition list back
// into the three model shapes the schema exposes as separate attributes.
func FlattenConditions(items []rules.ConditionItem) (conditions []ConditionModel, groups []GroupConditionModel, multivals []MultivalConditionModel) {
	for _, item := range items {
		switch item.Type {
		case "single":
			if sc, ok := item.Fields.(rules.SingleCondition); ok {
				conditions = append(conditions, flattenCondition(sc.Field, sc.Operator, sc.Value))
			}
		case "group":
			if gc, ok := item.Fields.(rules.GroupCondition); ok {
				groups = append(groups, flattenGroupCondition(gc))
			}
		case "multival":
			if mc, ok := item.Fields.(rules.MultivalCondition); ok {
				multivals = append(multivals, flattenMultivalCondition(mc))
			}
		}
	}
	return conditions, groups, multivals
}

func flattenCondition(field, operator, value string) ConditionModel {
	return ConditionModel{
		Field:    types.StringValue(field),
		Operator: types.StringValue(operator),
		Value:    types.StringValue(value),
	}
}

func flattenGroupCondition(gc rules.GroupCondition) GroupConditionModel {
	group := GroupConditionModel{
		GroupOperator: types.StringValue(gc.GroupOperator),
	}

	for _, item := range gc.Conditions {
		switch item.Type {
		case "single":
			if c, ok := item.Fields.(rules.Condition); ok {
				group.Condition = append(group.Condition, flattenCondition(c.Field, c.Operator, c.Value))
			}
		case "multival":
			if mc, ok := item.Fields.(rules.MultivalCondition); ok {
				group.MultivalCondition = append(group.MultivalCondition, flattenMultivalCondition(mc))
			}
		}
	}

	return group
}

func flattenMultivalCondition(mc rules.MultivalCondition) MultivalConditionModel {
	m := MultivalConditionModel{
		Field:         types.StringValue(mc.Field),
		Operator:      types.StringValue(mc.Operator),
		GroupOperator: types.StringValue(mc.GroupOperator),
	}

	for _, c := range mc.Conditions {
		m.Condition = append(m.Condition, flattenCondition(c.Field, c.Operator, c.Value))
	}

	return m
}
