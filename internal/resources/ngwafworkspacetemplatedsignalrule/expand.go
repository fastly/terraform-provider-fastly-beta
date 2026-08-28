package ngwafworkspacetemplatedsignalrule

import (
	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func BuildCreateInput(plan Model) *rules.CreateInput {
	input := ngwafrule.NewCreateInput(RuleType, plan.CommonModel)
	input.Actions = ngwafrule.ExpandCreateSignalActions(ActionType, plan.Action)
	return input
}
