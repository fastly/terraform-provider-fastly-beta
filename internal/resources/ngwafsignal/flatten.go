package ngwafsignal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

func FlattenToModel(ctx context.Context, signal *signals.Signal) (Model, error) {
	if signal == nil {
		return Model{}, fmt.Errorf("cannot flatten nil NGWAF signal")
	}
	if signal.SignalID == "" {
		return Model{}, fmt.Errorf("invalid NGWAF signal: id is missing")
	}
	if signal.Scope.Type == "" || len(signal.Scope.AppliesTo) == 0 {
		return Model{}, fmt.Errorf("invalid NGWAF signal scope: type or applies_to is missing")
	}
	if scope.Type(signal.Scope.Type) != scope.ScopeTypeAccount {
		return Model{}, fmt.Errorf("expected account-scoped NGWAF signal, got scope type %q", signal.Scope.Type)
	}

	appliesTo, diags := types.SetValueFrom(ctx, types.StringType, signal.Scope.AppliesTo)
	if diags.HasError() {
		return Model{}, fmt.Errorf("flattening signal applies_to: %v", diags)
	}

	return Model{
		ID:          types.StringValue(signal.SignalID),
		AppliesTo:   appliesTo,
		Name:        types.StringValue(signal.Name),
		Description: types.StringValue(signal.Description),
		ReferenceID: types.StringValue(signal.ReferenceID),
	}, nil
}
