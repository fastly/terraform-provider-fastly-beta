package ngwafworkspacesignalrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func BuildCreateInput(plan Model) *rules.CreateInput {
	input := ngwafrule.NewCreateInput(RuleType, plan.CommonModel)
	input.Description = new(service.StringValue(plan.Description))
	input.Actions = ngwafrule.ExpandCreateSignalActions(ActionType, plan.Action)
	return input
}

func BuildUpdateInput(ruleID string, plan Model) *rules.UpdateInput {
	input := ngwafrule.NewUpdateInput(RuleType, ruleID, plan.CommonModel)
	input.Description = new(service.StringValue(plan.Description))
	input.Actions = ngwafrule.ExpandUpdateSignalActions(ActionType, plan.Action)
	return input
}
