package ngwaflist

import "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"

// WorkspaceScope returns the NGWAF workspace scope for list API operations.
func WorkspaceScope(workspaceID string) *scope.Scope {
	return &scope.Scope{
		Type:      scope.ScopeTypeWorkspace,
		AppliesTo: []string{workspaceID},
	}
}
