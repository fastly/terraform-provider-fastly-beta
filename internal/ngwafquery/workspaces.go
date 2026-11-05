// Package ngwafquery holds helpers shared by the Next-Gen WAF list resources
// that back terraform query.
package ngwafquery

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/fastly/go-fastly/v19/fastly"
	ws "github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/workspaces"
)

// ListWorkspaces returns every workspace visible to the API token. Workspace
// scoped list resources discover their objects by iterating these.
func ListWorkspaces(ctx context.Context, client *fastly.Client) ([]ws.Workspace, diag.Diagnostics) {
	var diags diag.Diagnostics

	workspaces, err := ws.List(ctx, client, &ws.ListInput{})
	if err != nil {
		diags.AddError("Error listing NGWAF workspaces", err.Error())
		return nil, diags
	}

	return workspaces, diags
}
