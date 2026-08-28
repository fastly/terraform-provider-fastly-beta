package ngwafworkspaceratelimitrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func BuildCreateInput(plan Model) *rules.CreateInput {
	input := ngwafrule.NewCreateInput(RuleType, plan.CommonModel)
	input.Description = new(service.StringValue(plan.Description))
	input.Actions = ngwafrule.ExpandCreateActions(plan.Action)
	input.RateLimit = expandCreateRateLimit(plan.RateLimit)
	return input
}

func BuildUpdateInput(ruleID string, plan Model) *rules.UpdateInput {
	input := ngwafrule.NewUpdateInput(RuleType, ruleID, plan.CommonModel)
	input.Description = new(service.StringValue(plan.Description))
	input.Actions = ngwafrule.ExpandUpdateActions(plan.Action)
	input.RateLimit = expandUpdateRateLimit(plan.RateLimit)
	return input
}

func expandCreateRateLimit(models []RateLimitModel) *rules.CreateRateLimit {
	if len(models) == 0 {
		return nil
	}
	m := models[0]

	return &rules.CreateRateLimit{
		ClientIdentifiers: expandCreateClientIdentifiers(m.ClientIdentifiers),
		Duration:          ngwafrule.IntPointer(m.Duration),
		Interval:          ngwafrule.IntPointer(m.Interval),
		Signal:            m.Signal.ValueStringPointer(),
		Threshold:         ngwafrule.IntPointer(m.Threshold),
	}
}

func expandUpdateRateLimit(models []RateLimitModel) *rules.UpdateRateLimit {
	if len(models) == 0 {
		return nil
	}
	m := models[0]

	return &rules.UpdateRateLimit{
		ClientIdentifiers: expandUpdateClientIdentifiers(m.ClientIdentifiers),
		Duration:          ngwafrule.IntPointer(m.Duration),
		Interval:          ngwafrule.IntPointer(m.Interval),
		Signal:            m.Signal.ValueStringPointer(),
		Threshold:         ngwafrule.IntPointer(m.Threshold),
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
