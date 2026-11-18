package botmanagementbot

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

func BuildUpdateInput(m Model, action string) *policy.UpdateBotInput {
	return &policy.UpdateBotInput{
		WorkspaceID: new(service.StringValue(m.WorkspaceID)),
		CategoryID:  new(service.StringValue(m.CategoryID)),
		BotID:       new(service.StringValue(m.BotID)),
		Action:      &policy.Action{Type: action},
	}
}
