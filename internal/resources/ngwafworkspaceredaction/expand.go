package ngwafworkspaceredaction

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/redactions"
)

func BuildCreateInput(plan Model) *redactions.CreateInput {
	return &redactions.CreateInput{
		Field:       new(service.StringValue(plan.Field)),
		Type:        new(service.StringValue(plan.Type)),
		WorkspaceID: new(service.StringValue(plan.WorkspaceID)),
	}
}

func BuildUpdateInput(redactionID string, plan Model) *redactions.UpdateInput {
	return &redactions.UpdateInput{
		Field:       new(service.StringValue(plan.Field)),
		RedactionID: &redactionID,
		Type:        new(service.StringValue(plan.Type)),
		WorkspaceID: new(service.StringValue(plan.WorkspaceID)),
	}
}

func BuildGetInput(workspaceID, redactionID string) *redactions.GetInput {
	return &redactions.GetInput{
		RedactionID: &redactionID,
		WorkspaceID: &workspaceID,
	}
}

func BuildDeleteInput(workspaceID, redactionID string) *redactions.DeleteInput {
	return &redactions.DeleteInput{
		RedactionID: &redactionID,
		WorkspaceID: &workspaceID,
	}
}
