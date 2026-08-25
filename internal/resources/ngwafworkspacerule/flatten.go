package ngwafworkspacerule

import (
	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func FlattenToModel(rule *rules.Rule) Model {
	m := Model{
		ID:          types.StringValue(rule.RuleID),
		Type:        types.StringValue(rule.Type),
		Description: types.StringValue(rule.Description),
		Enabled:     types.BoolValue(rule.Enabled),
	}

	if len(rule.Scope.AppliesTo) > 0 {
		m.WorkspaceID = types.StringValue(rule.Scope.AppliesTo[0])
	}

	if rule.GroupOperator != "" {
		m.GroupOperator = types.StringValue(rule.GroupOperator)
	}
	if rule.RequestLogging != "" {
		m.RequestLogging = types.StringValue(rule.RequestLogging)
	}

	m.Condition, m.GroupCondition, m.MultivalCondition = ngwafrule.FlattenConditions(rule.Conditions)
	m.Action = flattenActions(rule.Actions)
	m.RateLimit = flattenRateLimit(rule.RateLimit)

	return m
}

func flattenActions(actions []rules.Action) []ActionModel {
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

func flattenRateLimit(rateLimit *rules.RateLimit) []RateLimitModel {
	if rateLimit == nil {
		return nil
	}

	clientIdentifiers := make([]ClientIdentifierModel, 0, len(rateLimit.ClientIdentifiers))
	for _, ci := range rateLimit.ClientIdentifiers {
		m := ClientIdentifierModel{
			Type: types.StringValue(ci.Type),
		}
		if ci.Key != "" {
			m.Key = types.StringValue(ci.Key)
		}
		if ci.Name != "" {
			m.Name = types.StringValue(ci.Name)
		}
		if ci.Signal != "" {
			m.Signal = types.StringValue(ci.Signal)
		}
		clientIdentifiers = append(clientIdentifiers, m)
	}

	return []RateLimitModel{{
		ClientIdentifiers: clientIdentifiers,
		Duration:          types.Int64Value(int64(rateLimit.Duration)),
		Interval:          types.Int64Value(int64(rateLimit.Interval)),
		Signal:            types.StringValue(rateLimit.Signal),
		Threshold:         types.Int64Value(int64(rateLimit.Threshold)),
	}}
}
