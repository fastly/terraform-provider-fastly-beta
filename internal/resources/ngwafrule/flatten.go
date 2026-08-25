package ngwafrule

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

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
