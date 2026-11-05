package ngwafalertintegration

import (
	"context"
	"fmt"

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
	def    Definition
}

func NewWorkspaceListResource(def Definition) list.ListResource {
	return &ListResource{def: def}
}

func (l *ListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_workspace_alert_" + l.def.TypeSuffix + "_integration"
}

func (l *ListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Description: fmt.Sprintf("List all Fastly Next-Gen WAF %s alert integrations across all workspaces accessible to the API token.", l.def.DisplayName),
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
	tflog.Debug(ctx, "Listing Fastly NGWAF workspace alert integrations", map[string]any{"type": l.def.Type})

	workspaces, diags := ngwafquery.ListWorkspaces(ctx, l.client)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		var count int64
		for _, w := range workspaces {
			alerts, err := l.def.Operations.List(ctx, l.client, w.WorkspaceID)
			if err != nil {
				tflog.Warn(ctx, "Error listing NGWAF alert integrations for workspace", map[string]any{
					"workspace_id": w.WorkspaceID,
					"type":         l.def.Type,
					"error":        err.Error(),
				})
				continue
			}

			for i := range alerts {
				alert := &alerts[i]
				if alert.ID == "" {
					continue
				}
				if req.Limit > 0 && count >= req.Limit {
					return
				}
				count++

				result := req.NewListResult(ctx)
				result.DisplayName = service.ToGeneratedResourceName(w.Name, l.def.TypeSuffix, alert.ID)
				result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.NGWAFWorkspaceObject(types.StringValue(w.WorkspaceID), types.StringValue(alert.ID)))...)

				if req.IncludeResource {
					model, err := FlattenToModel(l.def, w.WorkspaceID, alert)
					if err != nil {
						result.Diagnostics.AddError("Error reading NGWAF alert integration", err.Error())
					} else {
						result.Diagnostics.Append(SetModelState(ctx, result.Resource, l.def, model)...)
					}
				}

				if !push(result) {
					return
				}
			}
		}
	}
}
