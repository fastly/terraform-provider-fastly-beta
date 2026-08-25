package ngwafworkspacerule

import (
	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"
	"github.com/fastly/terraform-provider-fastly/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

func buildScope(workspaceID string) *scope.Scope {
	return &scope.Scope{
		Type:      scope.ScopeTypeWorkspace,
		AppliesTo: []string{workspaceID},
	}
}

func BuildCreateInput(plan Model) *rules.CreateInput {
	return &rules.CreateInput{
		Type:               new(service.StringValue(plan.Type)),
		Description:        new(service.StringValue(plan.Description)),
		Scope:              buildScope(service.StringValue(plan.WorkspaceID)),
		Enabled:            new(service.BoolValue(plan.Enabled)),
		GroupOperator:      plan.GroupOperator.ValueStringPointer(),
		RequestLogging:     plan.RequestLogging.ValueStringPointer(),
		Actions:            expandCreateActions(plan.Action),
		Conditions:         ngwafrule.ExpandConditions(plan.Condition, newCreateCondition),
		GroupConditions:    ngwafrule.ExpandGroupConditions(plan.GroupCondition, newCreateCondition, newCreateConditionMult, newCreateMultivalCondition, newCreateGroupCondition),
		MultivalConditions: ngwafrule.ExpandMultivalConditions(plan.MultivalCondition, newCreateConditionMult, newCreateMultivalCondition),
		RateLimit:          expandCreateRateLimit(plan.RateLimit),
	}
}

func BuildUpdateInput(ruleID string, plan Model) *rules.UpdateInput {
	input := &rules.UpdateInput{
		RuleID:             &ruleID,
		Scope:              buildScope(service.StringValue(plan.WorkspaceID)),
		Type:               new(service.StringValue(plan.Type)),
		Description:        new(service.StringValue(plan.Description)),
		Enabled:            new(service.BoolValue(plan.Enabled)),
		GroupOperator:      plan.GroupOperator.ValueStringPointer(),
		RequestLogging:     plan.RequestLogging.ValueStringPointer(),
		Conditions:         ngwafrule.ExpandConditions(plan.Condition, newUpdateCondition),
		GroupConditions:    ngwafrule.ExpandGroupConditions(plan.GroupCondition, newUpdateCondition, newUpdateConditionMult, newUpdateMultivalCondition, newUpdateGroupCondition),
		MultivalConditions: ngwafrule.ExpandMultivalConditions(plan.MultivalCondition, newUpdateConditionMult, newUpdateMultivalCondition),
		RateLimit:          expandUpdateRateLimit(plan.RateLimit),
	}

	// templated_signal updates must omit actions; ModifyPlan forces a
	// replace instead for any change on this rule type.
	if service.StringValue(plan.Type) != "templated_signal" {
		input.Actions = expandUpdateActions(plan.Action)
	}

	return input
}

func newCreateCondition(field, operator, value *string) *rules.CreateCondition {
	return &rules.CreateCondition{Field: field, Operator: operator, Value: value}
}

func newUpdateCondition(field, operator, value *string) *rules.UpdateCondition {
	return &rules.UpdateCondition{Field: field, Operator: operator, Value: value}
}

func newCreateConditionMult(field, operator, value *string) *rules.CreateConditionMult {
	return &rules.CreateConditionMult{Field: field, Operator: operator, Value: value}
}

func newUpdateConditionMult(field, operator, value *string) *rules.UpdateConditionMult {
	return &rules.UpdateConditionMult{Field: field, Operator: operator, Value: value}
}

func newCreateMultivalCondition(field, operator, groupOperator *string, conditions []*rules.CreateConditionMult) *rules.CreateMultivalCondition {
	return &rules.CreateMultivalCondition{Field: field, Operator: operator, GroupOperator: groupOperator, Conditions: conditions}
}

func newUpdateMultivalCondition(field, operator, groupOperator *string, conditions []*rules.UpdateConditionMult) *rules.UpdateMultivalCondition {
	return &rules.UpdateMultivalCondition{Field: field, Operator: operator, GroupOperator: groupOperator, Conditions: conditions}
}

func newCreateGroupCondition(groupOperator *string, conditions []*rules.CreateCondition, multivals []*rules.CreateMultivalCondition) *rules.CreateGroupCondition {
	return &rules.CreateGroupCondition{GroupOperator: groupOperator, Conditions: conditions, MultivalConditions: multivals}
}

func newUpdateGroupCondition(groupOperator *string, conditions []*rules.UpdateCondition, multivals []*rules.UpdateMultivalCondition) *rules.UpdateGroupCondition {
	return &rules.UpdateGroupCondition{GroupOperator: groupOperator, Conditions: conditions, MultivalConditions: multivals}
}

func expandCreateActions(models []ActionModel) []*rules.CreateAction {
	if len(models) == 0 {
		return nil
	}

	actions := make([]*rules.CreateAction, 0, len(models))
	for _, m := range models {
		actions = append(actions, &rules.CreateAction{
			Type:             m.Type.ValueStringPointer(),
			Signal:           m.Signal.ValueStringPointer(),
			AllowInteractive: m.AllowInteractive.ValueBoolPointer(),
			DeceptionType:    m.DeceptionType.ValueStringPointer(),
			RedirectURL:      m.RedirectURL.ValueStringPointer(),
			ResponseCode:     intPointer(m.ResponseCode),
		})
	}
	return actions
}

func expandUpdateActions(models []ActionModel) []*rules.UpdateAction {
	if len(models) == 0 {
		return nil
	}

	actions := make([]*rules.UpdateAction, 0, len(models))
	for _, m := range models {
		actions = append(actions, &rules.UpdateAction{
			Type:             m.Type.ValueStringPointer(),
			Signal:           m.Signal.ValueStringPointer(),
			AllowInteractive: m.AllowInteractive.ValueBoolPointer(),
			DeceptionType:    m.DeceptionType.ValueStringPointer(),
			RedirectURL:      m.RedirectURL.ValueStringPointer(),
			ResponseCode:     intPointer(m.ResponseCode),
		})
	}
	return actions
}

func expandCreateRateLimit(models []RateLimitModel) *rules.CreateRateLimit {
	if len(models) == 0 {
		return nil
	}
	m := models[0]

	return &rules.CreateRateLimit{
		ClientIdentifiers: expandCreateClientIdentifiers(m.ClientIdentifiers),
		Duration:          intPointer(m.Duration),
		Interval:          intPointer(m.Interval),
		Signal:            m.Signal.ValueStringPointer(),
		Threshold:         intPointer(m.Threshold),
	}
}

func expandUpdateRateLimit(models []RateLimitModel) *rules.UpdateRateLimit {
	if len(models) == 0 {
		return nil
	}
	m := models[0]

	return &rules.UpdateRateLimit{
		ClientIdentifiers: expandUpdateClientIdentifiers(m.ClientIdentifiers),
		Duration:          intPointer(m.Duration),
		Interval:          intPointer(m.Interval),
		Signal:            m.Signal.ValueStringPointer(),
		Threshold:         intPointer(m.Threshold),
	}
}

func expandCreateClientIdentifiers(models []ClientIdentifierModel) []*rules.CreateClientIdentifier {
	if len(models) == 0 {
		return nil
	}

	result := make([]*rules.CreateClientIdentifier, 0, len(models))
	for _, m := range models {
		result = append(result, &rules.CreateClientIdentifier{
			Key:    m.Key.ValueStringPointer(),
			Name:   m.Name.ValueStringPointer(),
			Signal: m.Signal.ValueString(),
			Type:   m.Type.ValueStringPointer(),
		})
	}
	return result
}

func expandUpdateClientIdentifiers(models []ClientIdentifierModel) []*rules.UpdateClientIdentifier {
	if len(models) == 0 {
		return nil
	}

	result := make([]*rules.UpdateClientIdentifier, 0, len(models))
	for _, m := range models {
		result = append(result, &rules.UpdateClientIdentifier{
			Key:    m.Key.ValueStringPointer(),
			Name:   m.Name.ValueStringPointer(),
			Signal: m.Signal.ValueString(),
			Type:   m.Type.ValueStringPointer(),
		})
	}
	return result
}

func intPointer(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
}
