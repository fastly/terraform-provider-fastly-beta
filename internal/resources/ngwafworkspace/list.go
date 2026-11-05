package ngwafworkspace

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafquery"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"
)

var (
	_ list.ListResource              = &ListResource{}
	_ list.ListResourceWithConfigure = &ListResource{}
)

type ListResource struct {
	client *fastly.Client
}

func NewListResource() list.ListResource {
	return &ListResource{}
}

func (l *ListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace"
}

func (l *ListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Description: "List all Fastly Next-Gen WAF workspaces accessible to the API token.",
	}
}

func (l *ListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	l.client = data.Client
}

func (l *ListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Debug(ctx, "Listing Fastly NGWAF workspaces")

	workspaces, diags := ngwafquery.ListWorkspaces(ctx, l.client)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		var count int64
		for i := range workspaces {
			workspace := &workspaces[i]
			if workspace.WorkspaceID == "" {
				continue
			}
			if req.Limit > 0 && count >= req.Limit {
				return
			}
			count++

			result := req.NewListResult(ctx)
			result.DisplayName = service.ToGeneratedResourceName(workspace.Name, workspace.WorkspaceID)
			result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.NGWAFWorkspace(types.StringValue(workspace.WorkspaceID)))...)

			if req.IncludeResource {
				model, diags := FlattenToModel(ctx, workspace)
				result.Diagnostics.Append(diags...)
				if !diags.HasError() {
					result.Diagnostics.Append(result.Resource.Set(ctx, &model)...)
				}
			}

			if !push(result) {
				return
			}
		}
	}
}
