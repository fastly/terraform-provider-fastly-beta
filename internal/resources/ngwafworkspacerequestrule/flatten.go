package ngwafworkspacerequestrule

import (
	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func FlattenToModel(rule *rules.Rule) Model {
	return Model{
		CommonModel:    ngwafrule.FlattenCommon(rule),
		Description:    types.StringValue(rule.Description),
		RequestLogging: types.StringValue(rule.RequestLogging),
		Action:         ngwafrule.FlattenActions(rule.Actions),
	}
}
