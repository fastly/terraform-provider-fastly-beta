package settings

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/listidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
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
	resp.TypeName = req.ProviderTypeName + "_service_settings"
}

func (l *ListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Description: "List general settings for all Fastly CDN services accessible to the API token, at their active version, or latest version when no active version exists.",
		Attributes:  map[string]listschema.Attribute{},
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
	tflog.Debug(ctx, "Listing Fastly service settings")

	services, err := l.client.ListServices(ctx, &fastly.ListServicesInput{})
	if err != nil {
		stream.Results = list.ListResultsStreamDiagnostics(diag.Diagnostics{
			diag.NewErrorDiagnostic("Error listing Fastly services", err.Error()),
		})
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		var count int64
		for _, svc := range services {
			// Settings is a VCL-only resource; see EnsureServiceTypeSupported in resource.go.
			if svc == nil || svc.Type == nil || *svc.Type != service.TypeVCL {
				continue
			}
			serviceID := fastly.ToValue(svc.ServiceID)
			if serviceID == "" {
				continue
			}

			version, _, err := service.SelectReadVersionFromServiceSummary(ctx, l.client, svc)
			if err != nil {
				tflog.Warn(ctx, "Error selecting service version for query", map[string]any{
					"service_id": serviceID,
					"error":      err.Error(),
				})
				continue
			}

			m, err := readCurrent(ctx, l.client, serviceID, version)
			if err != nil {
				tflog.Warn(ctx, "Error reading service settings for service", map[string]any{
					"service_id": serviceID,
					"error":      err.Error(),
				})
				continue
			}

			if req.Limit > 0 && count >= req.Limit {
				return
			}
			count++

			result := listidentity.NewResult(ctx, req)
			result.DisplayName = service.ToGeneratedResourceName(fastly.ToValue(svc.Name), serviceID)

			if req.IncludeResource {
				result.Diagnostics.Append(setResourceAttrs(ctx, &result, m, serviceID, version)...)
			}

			if !push(result) {
				return
			}
		}
	}
}

func setResourceAttrs(ctx context.Context, result *list.ListResult, m NestedModel, serviceID string, version int) diag.Diagnostics {
	var diags diag.Diagnostics

	model := Model{}
	flattenModel(&model, m, serviceID, version)
	diags.Append(result.Resource.Set(ctx, &model)...)
	return diags
}
