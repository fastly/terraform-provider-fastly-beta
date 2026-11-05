package ngwafworkspaceredaction

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafquery"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"

	"github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/workspaces/redactions"
)

func NewListResource() list.ListResource {
	return ngwafquery.NewListResource(ngwafquery.ListDefinition[redactions.Redaction]{
		TypeName: "_ngwaf_workspace_redaction",
		Noun:     "workspace redactions",
		List: func(ctx context.Context, client *fastly.Client, workspaceID string) ([]redactions.Redaction, error) {
			resp, err := redactions.List(ctx, client, &redactions.ListInput{WorkspaceID: &workspaceID})
			if err != nil || resp == nil {
				return nil, err
			}
			return resp.Data, nil
		},
		ID:          func(redaction *redactions.Redaction) string { return redaction.RedactionID },
		DisplayName: func(redaction *redactions.Redaction) []string { return []string{redaction.Field} },
		Flatten: func(_ context.Context, workspaceID string, redaction *redactions.Redaction) (any, diag.Diagnostics) {
			model := FlattenToModel(workspaceID, redaction)
			return &model, nil
		},
	})
}
