package ngwafworkspacerequestrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func BuildCreateInput(plan Model) *rules.CreateInput {
	input := ngwafrule.NewCreateInput(RuleType, ngwafrule.WorkspaceScope(service.StringValue(plan.WorkspaceID)), plan.CommonModel)
	input.Description = new(service.StringValue(plan.Description))
	input.RequestLogging = plan.RequestLogging.ValueStringPointer()
	input.Actions = ngwafrule.ExpandCreateActions(plan.Action)
	return input
}

func BuildUpdateInput(ruleID string, plan Model) *rules.UpdateInput {
	input := ngwafrule.NewUpdateInput(RuleType, ruleID, ngwafrule.WorkspaceScope(service.StringValue(plan.WorkspaceID)), plan.CommonModel)
	input.Description = new(service.StringValue(plan.Description))
	input.RequestLogging = plan.RequestLogging.ValueStringPointer()
	input.Actions = ngwafrule.ExpandUpdateActions(plan.Action)
	return input
}
