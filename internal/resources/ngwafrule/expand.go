package ngwafrule

// ExpandConditions builds the API's flat condition list from its model
// form. C is the scope- and operation-specific condition type (e.g.
// rules.CreateCondition, rules.UpdateCondition, rules.CreateConditionMult) -
// every one of those types shares the same Field/Operator/Value shape, so
// callers only need to supply the right constructor.
func ExpandConditions[C any](models []ConditionModel, newCondition func(field, operator, value *string) *C) []*C {
	if len(models) == 0 {
		return nil
	}

	result := make([]*C, 0, len(models))
	for _, m := range models {
		result = append(result, newCondition(
			m.Field.ValueStringPointer(),
			m.Operator.ValueStringPointer(),
			m.Value.ValueStringPointer(),
		))
	}
	return result
}

// ExpandMultivalConditions builds the API's multival condition list. M is
// the scope- and operation-specific multival type (e.g.
// rules.CreateMultivalCondition); MC is the type of its nested condition
// list (e.g. rules.CreateConditionMult).
func ExpandMultivalConditions[M, MC any](
	models []MultivalConditionModel,
	newCondition func(field, operator, value *string) *MC,
	newMultival func(field, operator, groupOperator *string, conditions []*MC) *M,
) []*M {
	if len(models) == 0 {
		return nil
	}

	result := make([]*M, 0, len(models))
	for _, m := range models {
		result = append(result, newMultival(
			m.Field.ValueStringPointer(),
			m.Operator.ValueStringPointer(),
			m.GroupOperator.ValueStringPointer(),
			ExpandConditions(m.Condition, newCondition),
		))
	}
	return result
}

// ExpandGroupConditions builds the API's group condition list. G is the
// scope- and operation-specific group type (e.g. rules.CreateGroupCondition);
// C is its nested condition type; M is its nested multival type; MC is that
// multival's own nested condition type.
func ExpandGroupConditions[G, C, M, MC any](
	models []GroupConditionModel,
	newCondition func(field, operator, value *string) *C,
	newMultivalCondition func(field, operator, value *string) *MC,
	newMultival func(field, operator, groupOperator *string, conditions []*MC) *M,
	newGroup func(groupOperator *string, conditions []*C, multivals []*M) *G,
) []*G {
	if len(models) == 0 {
		return nil
	}

	result := make([]*G, 0, len(models))
	for _, m := range models {
		result = append(result, newGroup(
			m.GroupOperator.ValueStringPointer(),
			ExpandConditions(m.Condition, newCondition),
			ExpandMultivalConditions(m.MultivalCondition, newMultivalCondition, newMultival),
		))
	}
	return result
}
