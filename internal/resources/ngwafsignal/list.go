package ngwafsignal

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafquery"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"

	"github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/signals"
)

func NewListResource() list.ListResource {
	return ngwafquery.NewListResource(ngwafquery.ListDefinition[signals.Signal]{
		TypeName: "_ngwaf_signal",
		Noun:     "signals",
		Account:  true,
		List: func(ctx context.Context, client *fastly.Client, _ string) ([]signals.Signal, error) {
			resp, err := signals.List(ctx, client, &signals.ListInput{Scope: &scope.Scope{Type: scope.ScopeTypeAccount}})
			if err != nil || resp == nil {
				return nil, err
			}

			var out []signals.Signal
			for _, signal := range resp.Data {
				if scope.Type(signal.Scope.Type) == scope.ScopeTypeAccount {
					out = append(out, signal)
				}
			}
			return out, nil
		},
		ID:          func(signal *signals.Signal) string { return signal.SignalID },
		DisplayName: func(signal *signals.Signal) []string { return []string{signal.Name} },
		Flatten: func(ctx context.Context, _ string, signal *signals.Signal) (any, diag.Diagnostics) {
			var diags diag.Diagnostics
			model, err := FlattenToModel(ctx, signal)
			if err != nil {
				diags.AddError("Error reading NGWAF signal", err.Error())
			}
			return &model, diags
		},
	})
}
