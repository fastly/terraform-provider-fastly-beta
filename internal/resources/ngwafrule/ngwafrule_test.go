package ngwafrule

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func TestExpandConditions(t *testing.T) {
	models := []ConditionModel{
		{Field: types.StringValue("ip"), Operator: types.StringValue("equals"), Value: types.StringValue("1.2.3.4")},
	}

	result := ExpandConditions(models, func(field, operator, value *string) *rules.CreateCondition {
		return &rules.CreateCondition{Field: field, Operator: operator, Value: value}
	})

	if assert.Len(t, result, 1) {
		assert.Equal(t, "ip", *result[0].Field)
		assert.Equal(t, "equals", *result[0].Operator)
		assert.Equal(t, "1.2.3.4", *result[0].Value)
	}
}

func TestExpandConditions_nilOnEmpty(t *testing.T) {
	result := ExpandConditions[rules.CreateCondition](nil, func(field, operator, value *string) *rules.CreateCondition {
		return &rules.CreateCondition{Field: field, Operator: operator, Value: value}
	})
	assert.Nil(t, result)
}

func TestExpandMultivalConditions(t *testing.T) {
	models := []MultivalConditionModel{
		{
			Field:         types.StringValue("post_parameter"),
			Operator:      types.StringValue("exists"),
			GroupOperator: types.StringValue("any"),
			Condition: []ConditionModel{
				{Field: types.StringValue("name"), Operator: types.StringValue("equals"), Value: types.StringValue("foo")},
			},
		},
	}

	newCondition := func(field, operator, value *string) *rules.CreateConditionMult {
		return &rules.CreateConditionMult{Field: field, Operator: operator, Value: value}
	}
	newMultival := func(field, operator, groupOperator *string, conditions []*rules.CreateConditionMult) *rules.CreateMultivalCondition {
		return &rules.CreateMultivalCondition{Field: field, Operator: operator, GroupOperator: groupOperator, Conditions: conditions}
	}

	result := ExpandMultivalConditions(models, newCondition, newMultival)

	if assert.Len(t, result, 1) {
		assert.Equal(t, "post_parameter", *result[0].Field)
		assert.Equal(t, "any", *result[0].GroupOperator)
		if assert.Len(t, result[0].Conditions, 1) {
			assert.Equal(t, "name", *result[0].Conditions[0].Field)
		}
	}
}

func TestExpandGroupConditions(t *testing.T) {
	models := []GroupConditionModel{
		{
			GroupOperator: types.StringValue("all"),
			Condition: []ConditionModel{
				{Field: types.StringValue("ip"), Operator: types.StringValue("equals"), Value: types.StringValue("1.2.3.4")},
			},
			MultivalCondition: []MultivalConditionModel{
				{
					Field:         types.StringValue("request_header"),
					Operator:      types.StringValue("does_not_exist"),
					GroupOperator: types.StringValue("all"),
					Condition: []ConditionModel{
						{Field: types.StringValue("name"), Operator: types.StringValue("equals"), Value: types.StringValue("X-Foo")},
					},
				},
			},
		},
	}

	newCondition := func(field, operator, value *string) *rules.CreateCondition {
		return &rules.CreateCondition{Field: field, Operator: operator, Value: value}
	}
	newMultivalCondition := func(field, operator, value *string) *rules.CreateConditionMult {
		return &rules.CreateConditionMult{Field: field, Operator: operator, Value: value}
	}
	newMultival := func(field, operator, groupOperator *string, conditions []*rules.CreateConditionMult) *rules.CreateMultivalCondition {
		return &rules.CreateMultivalCondition{Field: field, Operator: operator, GroupOperator: groupOperator, Conditions: conditions}
	}
	newGroup := func(groupOperator *string, conditions []*rules.CreateCondition, multivals []*rules.CreateMultivalCondition) *rules.CreateGroupCondition {
		return &rules.CreateGroupCondition{GroupOperator: groupOperator, Conditions: conditions, MultivalConditions: multivals}
	}

	result := ExpandGroupConditions(models, newCondition, newMultivalCondition, newMultival, newGroup)

	if assert.Len(t, result, 1) {
		assert.Equal(t, "all", *result[0].GroupOperator)
		assert.Len(t, result[0].Conditions, 1)
		if assert.Len(t, result[0].MultivalConditions, 1) {
			assert.Equal(t, "request_header", *result[0].MultivalConditions[0].Field)
		}
	}
}

func TestFlattenConditions(t *testing.T) {
	items := []rules.ConditionItem{
		{
			Type: "single",
			Fields: rules.SingleCondition{
				Field: "ip", Operator: "equals", Value: "1.2.3.4",
			},
		},
		{
			Type: "group",
			Fields: rules.GroupCondition{
				GroupOperator: "any",
				Conditions: []rules.GroupConditionItem{
					{Type: "single", Fields: rules.Condition{Field: "path", Operator: "contains", Value: "/admin"}},
					{
						Type: "multival",
						Fields: rules.MultivalCondition{
							Field: "post_parameter", Operator: "exists", GroupOperator: "all",
							Conditions: []rules.ConditionMul{{Field: "name", Operator: "equals", Value: "foo"}},
						},
					},
				},
			},
		},
		{
			Type: "multival",
			Fields: rules.MultivalCondition{
				Field: "request_header", Operator: "does_not_exist", GroupOperator: "any",
				Conditions: []rules.ConditionMul{{Field: "name", Operator: "equals", Value: "X-Foo"}},
			},
		},
	}

	conditions, groups, multivals := FlattenConditions(items)

	if assert.Len(t, conditions, 1) {
		assert.Equal(t, types.StringValue("ip"), conditions[0].Field)
	}

	if assert.Len(t, groups, 1) {
		assert.Equal(t, types.StringValue("any"), groups[0].GroupOperator)
		if assert.Len(t, groups[0].Condition, 1) {
			assert.Equal(t, types.StringValue("path"), groups[0].Condition[0].Field)
		}
		if assert.Len(t, groups[0].MultivalCondition, 1) {
			assert.Equal(t, types.StringValue("post_parameter"), groups[0].MultivalCondition[0].Field)
		}
	}

	if assert.Len(t, multivals, 1) {
		assert.Equal(t, types.StringValue("request_header"), multivals[0].Field)
		if assert.Len(t, multivals[0].Condition, 1) {
			assert.Equal(t, types.StringValue("X-Foo"), multivals[0].Condition[0].Value)
		}
	}
}

func TestHasAnyCondition(t *testing.T) {
	assert.False(t, HasAnyCondition(nil, nil, nil))
	assert.True(t, HasAnyCondition([]ConditionModel{{}}, nil, nil))
	assert.True(t, HasAnyCondition(nil, []GroupConditionModel{{}}, nil))
	assert.True(t, HasAnyCondition(nil, nil, []MultivalConditionModel{{}}))
}

func TestEmptyGroupConditionIndexes(t *testing.T) {
	groups := []GroupConditionModel{
		{Condition: []ConditionModel{{}}},
		{},
		{MultivalCondition: []MultivalConditionModel{{}}},
	}

	assert.Equal(t, []int{1}, EmptyGroupConditionIndexes(groups))
}
