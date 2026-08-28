package ngwafworkspacesignalrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func FlattenToModel(rule *rules.Rule) Model {
	return Model{
		CommonModel: ngwafrule.FlattenCommon(rule),
		Description: types.StringValue(rule.Description),
		Action:      ngwafrule.FlattenSignalActions(rule.Actions),
	}
}
