package ngwafrule

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

// WorkspaceScope scopes a rule request to a single workspace.
func WorkspaceScope(workspaceID string) *scope.Scope {
	return &scope.Scope{
		Type:      scope.ScopeTypeWorkspace,
		AppliesTo: []string{workspaceID},
	}
}

// AccountScope scopes a rule request to the account, applying it to the given
// workspace IDs - or to every workspace in the account when appliesTo is the
// single entry "*". Create and update send this in the request body, which is
// the only way a rule's workspace list is set.
func AccountScope(appliesTo []string) *scope.Scope {
	return &scope.Scope{
		Type:      scope.ScopeTypeAccount,
		AppliesTo: appliesTo,
	}
}

// AccountScopeByID scopes a request to the account for the operations
// addressed by rule ID alone - get and delete. Those derive their path from
// the scope type only (/ngwaf/v1/rules/{rule_id}), so the workspace list plays
// no part in them.
func AccountScopeByID() *scope.Scope {
	return &scope.Scope{Type: scope.ScopeTypeAccount}
}

// ExpandAppliesTo reads the account-scope workspace list out of its set form.
func ExpandAppliesTo(ctx context.Context, appliesTo types.Set) ([]string, diag.Diagnostics) {
	var result []string
	diags := appliesTo.ElementsAs(ctx, &result, false)
	return result, diags
}

// NewCreateInput builds the create input fields shared by every rule type at
// either scope. ruleType is the calling resource's fixed `type` value and sc
// its scope; the caller fills in the rest.
func NewCreateInput(ruleType string, sc *scope.Scope, m CommonModel) *rules.CreateInput {
	return &rules.CreateInput{
		Type:               &ruleType,
		Scope:              sc,
		Enabled:            new(service.BoolValue(m.Enabled)),
		GroupOperator:      m.GroupOperator.ValueStringPointer(),
		Conditions:         ExpandConditions(m.Condition, newCreateCondition),
		GroupConditions:    ExpandGroupConditions(m.GroupCondition, newCreateCondition, newCreateConditionMult, newCreateMultivalCondition, newCreateGroupCondition),
		MultivalConditions: ExpandMultivalConditions(m.MultivalCondition, newCreateConditionMult, newCreateMultivalCondition),
	}
}

// NewUpdateInput builds the update input fields shared by every rule type at
// either scope.
func NewUpdateInput(ruleType, ruleID string, sc *scope.Scope, m CommonModel) *rules.UpdateInput {
	return &rules.UpdateInput{
		RuleID:             &ruleID,
		Type:               &ruleType,
		Scope:              sc,
		Enabled:            new(service.BoolValue(m.Enabled)),
		GroupOperator:      m.GroupOperator.ValueStringPointer(),
		Conditions:         ExpandConditions(m.Condition, newUpdateCondition),
		GroupConditions:    ExpandGroupConditions(m.GroupCondition, newUpdateCondition, newUpdateConditionMult, newUpdateMultivalCondition, newUpdateGroupCondition),
		MultivalConditions: ExpandMultivalConditions(m.MultivalCondition, newUpdateConditionMult, newUpdateMultivalCondition),
	}
}

// ExpandCreateActions builds the API's action list from the multi-shape
// ActionModel.
func ExpandCreateActions(models []ActionModel) []*rules.CreateAction {
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
			ResponseCode:     IntPointer(m.ResponseCode),
		})
	}
	return actions
}

// ExpandUpdateActions builds the API's action list from the multi-shape
// ActionModel.
func ExpandUpdateActions(models []ActionModel) []*rules.UpdateAction {
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
			ResponseCode:     IntPointer(m.ResponseCode),
		})
	}
	return actions
}

// ExpandCreateAccountActions builds the API's action list from
// AccountActionModel, which carries no custom-response fields.
func ExpandCreateAccountActions(models []AccountActionModel) []*rules.CreateAction {
	if len(models) == 0 {
		return nil
	}

	actions := make([]*rules.CreateAction, 0, len(models))
	for _, m := range models {
		actions = append(actions, &rules.CreateAction{
			Type:   m.Type.ValueStringPointer(),
			Signal: m.Signal.ValueStringPointer(),
		})
	}
	return actions
}

// ExpandUpdateAccountActions builds the API's action list from
// AccountActionModel, which carries no custom-response fields.
func ExpandUpdateAccountActions(models []AccountActionModel) []*rules.UpdateAction {
	if len(models) == 0 {
		return nil
	}

	actions := make([]*rules.UpdateAction, 0, len(models))
	for _, m := range models {
		actions = append(actions, &rules.UpdateAction{
			Type:   m.Type.ValueStringPointer(),
			Signal: m.Signal.ValueStringPointer(),
		})
	}
	return actions
}

// ExpandCreateSignalActions builds the API's action list from
// SignalActionModel, using the caller-supplied actionType since
// SignalActionModel has no type field of its own.
func ExpandCreateSignalActions(actionType string, models []SignalActionModel) []*rules.CreateAction {
	if len(models) == 0 {
		return nil
	}

	actions := make([]*rules.CreateAction, 0, len(models))
	for _, m := range models {
		actions = append(actions, &rules.CreateAction{
			Type:   &actionType,
			Signal: m.Signal.ValueStringPointer(),
		})
	}
	return actions
}

// ExpandUpdateSignalActions builds the API's action list from
// SignalActionModel, using the caller-supplied actionType since
// SignalActionModel has no type field of its own.
func ExpandUpdateSignalActions(actionType string, models []SignalActionModel) []*rules.UpdateAction {
	if len(models) == 0 {
		return nil
	}

	actions := make([]*rules.UpdateAction, 0, len(models))
	for _, m := range models {
		actions = append(actions, &rules.UpdateAction{
			Type:   &actionType,
			Signal: m.Signal.ValueStringPointer(),
		})
	}
	return actions
}

// IntPointer converts an Int64 attribute to the *int the API client takes,
// leaving it nil when unset.
func IntPointer(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
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
