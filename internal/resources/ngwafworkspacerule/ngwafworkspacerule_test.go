package ngwafworkspacerule

import (
	"testing"

	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		WorkspaceID: types.StringValue("ws-1"),
		Type:        types.StringValue("request"),
		Description: types.StringValue("desc"),
		Enabled:     types.BoolValue(true),
		Condition: []ngwafrule.ConditionModel{
			{Field: types.StringValue("ip"), Operator: types.StringValue("equals"), Value: types.StringValue("1.2.3.4")},
		},
		Action: []ActionModel{
			{Type: types.StringValue("block"), RedirectURL: types.StringValue("https://example.com"), ResponseCode: types.Int64Value(302)},
		},
	}

	input := BuildCreateInput(plan)

	assert.Equal(t, "request", *input.Type)
	assert.Equal(t, "desc", *input.Description)
	assert.True(t, *input.Enabled)
	assert.Equal(t, "workspace", string(input.Scope.Type))
	assert.Equal(t, []string{"ws-1"}, input.Scope.AppliesTo)
	if assert.Len(t, input.Conditions, 1) {
		assert.Equal(t, "ip", *input.Conditions[0].Field)
	}
	if assert.Len(t, input.Actions, 1) {
		assert.Equal(t, "block", *input.Actions[0].Type)
		assert.Equal(t, "https://example.com", *input.Actions[0].RedirectURL)
		assert.Equal(t, 302, *input.Actions[0].ResponseCode)
	}
}

func TestBuildUpdateInput_templatedSignalOmitsActions(t *testing.T) {
	plan := Model{
		WorkspaceID: types.StringValue("ws-1"),
		Type:        types.StringValue("templated_signal"),
		Description: types.StringValue(""),
		Enabled:     types.BoolValue(true),
		Condition: []ngwafrule.ConditionModel{
			{Field: types.StringValue("ip"), Operator: types.StringValue("equals"), Value: types.StringValue("1.2.3.4")},
		},
		Action: []ActionModel{
			{Type: types.StringValue("block")},
		},
	}

	input := BuildUpdateInput("rule-1", plan)

	assert.Equal(t, "rule-1", *input.RuleID)
	assert.Nil(t, input.Actions, "templated_signal updates must never include actions")
}

func TestBuildUpdateInput_nonTemplatedSignalIncludesActions(t *testing.T) {
	plan := Model{
		WorkspaceID: types.StringValue("ws-1"),
		Type:        types.StringValue("request"),
		Description: types.StringValue("desc"),
		Enabled:     types.BoolValue(true),
		Action: []ActionModel{
			{Type: types.StringValue("allow")},
		},
	}

	input := BuildUpdateInput("rule-1", plan)

	if assert.Len(t, input.Actions, 1) {
		assert.Equal(t, "allow", *input.Actions[0].Type)
	}
}

func TestBuildCreateInput_rateLimit(t *testing.T) {
	plan := Model{
		WorkspaceID: types.StringValue("ws-1"),
		Type:        types.StringValue("rate_limit"),
		Description: types.StringValue("desc"),
		Enabled:     types.BoolValue(true),
		RateLimit: []RateLimitModel{
			{
				Duration:  types.Int64Value(60),
				Interval:  types.Int64Value(60),
				Signal:    types.StringValue("sig-1"),
				Threshold: types.Int64Value(100),
				ClientIdentifiers: []ClientIdentifierModel{
					{Type: types.StringValue("ip")},
				},
			},
		},
	}

	input := BuildCreateInput(plan)

	if assert.NotNil(t, input.RateLimit) {
		assert.Equal(t, 60, *input.RateLimit.Duration)
		assert.Equal(t, 100, *input.RateLimit.Threshold)
		if assert.Len(t, input.RateLimit.ClientIdentifiers, 1) {
			assert.Equal(t, "ip", *input.RateLimit.ClientIdentifiers[0].Type)
		}
	}
}

func TestFlattenToModel(t *testing.T) {
	allowInteractive := true
	rule := &rules.Rule{
		RuleID:      "rule-1",
		Type:        "request",
		Description: "desc",
		Enabled:     true,
		Scope:       rules.Scope{Type: "workspace", AppliesTo: []string{"ws-1"}},
		Conditions: []rules.ConditionItem{
			{Type: "single", Fields: rules.SingleCondition{Field: "ip", Operator: "equals", Value: "1.2.3.4"}},
		},
		Actions: []rules.Action{
			{Type: "browser_challenge", AllowInteractive: &allowInteractive},
		},
	}

	m := FlattenToModel(rule)

	assert.Equal(t, types.StringValue("rule-1"), m.ID)
	assert.Equal(t, types.StringValue("ws-1"), m.WorkspaceID)
	assert.Equal(t, types.StringValue("request"), m.Type)
	if assert.Len(t, m.Condition, 1) {
		assert.Equal(t, types.StringValue("ip"), m.Condition[0].Field)
	}
	if assert.Len(t, m.Action, 1) {
		assert.Equal(t, types.StringValue("browser_challenge"), m.Action[0].Type)
		assert.Equal(t, types.BoolValue(true), m.Action[0].AllowInteractive)
	}
}

func TestFlattenToModel_rateLimit(t *testing.T) {
	rule := &rules.Rule{
		RuleID:  "rule-1",
		Type:    "rate_limit",
		Scope:   rules.Scope{Type: "workspace", AppliesTo: []string{"ws-1"}},
		Enabled: true,
		RateLimit: &rules.RateLimit{
			Duration:  60,
			Interval:  600,
			Signal:    "sig-1",
			Threshold: 100,
			ClientIdentifiers: []rules.ClientIdentifier{
				{Type: "ip"},
			},
		},
	}

	m := FlattenToModel(rule)

	if assert.Len(t, m.RateLimit, 1) {
		assert.Equal(t, types.Int64Value(60), m.RateLimit[0].Duration)
		assert.Equal(t, types.Int64Value(600), m.RateLimit[0].Interval)
		if assert.Len(t, m.RateLimit[0].ClientIdentifiers, 1) {
			assert.Equal(t, types.StringValue("ip"), m.RateLimit[0].ClientIdentifiers[0].Type)
		}
	}
}
