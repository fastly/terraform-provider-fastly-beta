package ngwafworkspacesignal

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafquery"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"

	"github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/scope"
	sig "github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/signals"
)

func NewListResource() list.ListResource {
	return ngwafquery.NewListResource(ngwafquery.ListDefinition[sig.Signal]{
		TypeName: "_ngwaf_workspace_signal",
		Noun:     "workspace signals",
		List: func(ctx context.Context, client *fastly.Client, workspaceID string) ([]sig.Signal, error) {
			resp, err := sig.List(ctx, client, &sig.ListInput{Scope: &scope.Scope{
				Type:      scope.ScopeTypeWorkspace,
				AppliesTo: []string{workspaceID},
			}})
			if err != nil || resp == nil {
				return nil, err
			}

			var out []sig.Signal
			for _, signal := range resp.Data {
				if scope.Type(signal.Scope.Type) == scope.ScopeTypeWorkspace {
					out = append(out, signal)
				}
			}
			return out, nil
		},
		ID:          func(signal *sig.Signal) string { return signal.SignalID },
		DisplayName: func(signal *sig.Signal) []string { return []string{signal.Name} },
		Flatten: func(_ context.Context, _ string, signal *sig.Signal) (any, diag.Diagnostics) {
			var diags diag.Diagnostics
			model, err := FlattenToModel(signal)
			if err != nil {
				diags.AddError("Error reading NGWAF workspace signal", err.Error())
			}
			return &model, diags
		},
	})
}
