package ngwafworkspaceratelimitrule

import (
	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func FlattenToModel(rule *rules.Rule) Model {
	return Model{
		CommonModel: ngwafrule.FlattenCommon(rule),
		Description: types.StringValue(rule.Description),
		Action:      ngwafrule.FlattenActions(rule.Actions),
		RateLimit:   flattenRateLimit(rule.RateLimit),
	}
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
