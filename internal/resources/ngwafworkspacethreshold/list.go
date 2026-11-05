package ngwafworkspacethreshold

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafquery"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"

	"github.com/fastly/go-fastly/v19/fastly"
	th "github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/workspaces/thresholds"
)

func NewListResource() list.ListResource {
	return ngwafquery.NewListResource(ngwafquery.ListDefinition[th.Threshold]{
		TypeName: "_ngwaf_workspace_threshold",
		Noun:     "workspace thresholds",
		List: func(ctx context.Context, client *fastly.Client, workspaceID string) ([]th.Threshold, error) {
			resp, err := th.List(ctx, client, &th.ListInput{WorkspaceID: &workspaceID})
			if err != nil || resp == nil {
				return nil, err
			}
			return resp.Data, nil
		},
		ID:          func(threshold *th.Threshold) string { return threshold.ThresholdID },
		DisplayName: func(threshold *th.Threshold) []string { return []string{threshold.Name} },
		Flatten: func(_ context.Context, workspaceID string, threshold *th.Threshold) (any, diag.Diagnostics) {
			model := FlattenToModel(workspaceID, threshold)
			return &model, nil
		},
	})
}
