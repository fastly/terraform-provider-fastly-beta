package logging

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v18/fastly"
)

// ListConfig describes how to list one kind of logging endpoint. API is the
// go-fastly endpoint type (e.g. fastly.Kafka).
type ListConfig[API any] struct {
	// TypeSuffix is appended to the provider type name, e.g.
	// "_service_logging_kafka".
	TypeSuffix string
	// Label is the human-readable endpoint kind used in the schema description
	// and log messages, e.g. "Kafka".
	Label string
	// List returns every endpoint of this kind on the given service version.
	List func(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*API, error)
	// Name returns the endpoint's name, or nil when unset.
	Name func(*API) *string
	// SetAttrs sets the listed resource's attributes on result.
	SetAttrs func(ctx context.Context, result *list.ListResult, api *API, serviceID string, version int, serviceType string) diag.Diagnostics
}

type listResource[API any] struct {
	cfg    ListConfig[API]
	client *fastly.Client
}

// NewListResource returns a list.ListResource for the logging endpoint kind
// described by cfg.
func NewListResource[API any](cfg ListConfig[API]) list.ListResource {
	return &listResource[API]{cfg: cfg}
}

var (
	_ list.ListResource              = &listResource[struct{}]{}
	_ list.ListResourceWithConfigure = &listResource[struct{}]{}
)

func (l *listResource[API]) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + l.cfg.TypeSuffix
}

func (l *listResource[API]) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = service.QueryListResourceConfigSchema("List all " + l.cfg.Label + " logging endpoints across all Fastly CDN and Compute services at their active version, or latest version when no active version exists.")
}

func (l *listResource[API]) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	l.client = data.Client
}

func (l *listResource[API]) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Debug(ctx, "Listing Fastly "+l.cfg.Label+" logging endpoints")

	services, diags := service.ListServicesForQuery(ctx, l.client, req)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		var count int64
		for _, svc := range services {
			if svc == nil || svc.Type == nil || !service.TypeSupported(*svc.Type, service.TypeVCL, service.TypeCompute) {
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

			loggers, err := l.cfg.List(ctx, l.client, serviceID, version)
			if err != nil {
				tflog.Warn(ctx, "Error listing "+l.cfg.Label+" logging endpoints for service", map[string]any{
					"service_id": serviceID,
					"error":      err.Error(),
				})
				continue
			}

			for _, api := range loggers {
				if api == nil || l.cfg.Name(api) == nil {
					continue
				}
				if req.Limit > 0 && count >= req.Limit {
					return
				}
				count++

				name := *l.cfg.Name(api)
				result := req.NewListResult(ctx)
				result.DisplayName = service.ToGeneratedResourceName(fastly.ToValue(svc.Name), serviceID, name)
				result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.NamedVersioned(types.StringValue(serviceID), types.StringValue(name)))...)

				if req.IncludeResource {
					result.Diagnostics.Append(l.cfg.SetAttrs(ctx, &result, api, serviceID, version, fastly.ToValue(svc.Type))...)
				}

				if !push(result) {
					return
				}
			}
		}
	}
}
