package ngwafworkspacesignal

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

func workspaceScope(workspaceID string) *scope.Scope {
	return &scope.Scope{
		Type:      scope.ScopeTypeWorkspace,
		AppliesTo: []string{workspaceID},
	}
}

func BuildCreateInput(plan Model) *signals.CreateInput {
	workspaceID := service.StringValue(plan.WorkspaceID)

	return &signals.CreateInput{
		Name:        new(service.StringValue(plan.Name)),
		Description: new(service.StringValue(plan.Description)),
		Scope:       workspaceScope(workspaceID),
	}
}

func BuildUpdateInput(signalID string, plan Model) *signals.UpdateInput {
	workspaceID := service.StringValue(plan.WorkspaceID)

	return &signals.UpdateInput{
		SignalID:    &signalID,
		Description: new(service.StringValue(plan.Description)),
		Scope:       workspaceScope(workspaceID),
	}
}

func BuildGetInput(workspaceID, signalID string) *signals.GetInput {
	return &signals.GetInput{
		SignalID: &signalID,
		Scope:    workspaceScope(workspaceID),
	}
}

func BuildDeleteInput(workspaceID, signalID string) *signals.DeleteInput {
	return &signals.DeleteInput{
		SignalID: &signalID,
		Scope:    workspaceScope(workspaceID),
	}
}
