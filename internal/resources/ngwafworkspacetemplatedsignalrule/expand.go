package ngwafworkspacetemplatedsignalrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func BuildCreateInput(plan Model) *rules.CreateInput {
	input := ngwafrule.NewCreateInput(RuleType, ngwafrule.WorkspaceScope(service.StringValue(plan.WorkspaceID)), plan.CommonModel)
	input.Actions = ngwafrule.ExpandCreateSignalActions(ActionType, plan.Action)
	return input
}
