package ngwafworkspacesignal

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	sig "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

func FlattenToModel(signal *sig.Signal) (Model, error) {
	if signal == nil {
		return Model{}, fmt.Errorf("cannot flatten nil NGWAF workspace signal")
	}
	if signal.SignalID == "" {
		return Model{}, fmt.Errorf("invalid NGWAF workspace signal: id is missing")
	}
	if signal.Scope.Type == "" || len(signal.Scope.AppliesTo) == 0 {
		return Model{}, fmt.Errorf("invalid NGWAF signal scope: type or applies_to is missing")
	}
	if scope.Type(signal.Scope.Type) != scope.ScopeTypeWorkspace {
		return Model{}, fmt.Errorf("expected workspace-scoped NGWAF signal, got scope type %q", signal.Scope.Type)
	}

	return Model{
		ID:          types.StringValue(signal.SignalID),
		WorkspaceID: types.StringValue(signal.Scope.AppliesTo[0]),
		Name:        types.StringValue(signal.Name),
		Description: types.StringValue(signal.Description),
		ReferenceID: types.StringValue(signal.ReferenceID),
	}, nil
}
