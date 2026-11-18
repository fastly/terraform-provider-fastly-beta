package loggingfos

import (
	"context"
	"fmt"

	"github.com/fastly/terraform-provider-fastly-beta/internal/logging"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v19/fastly"
)

func NewListResource() list.ListResource {
	return logging.NewListResource(logging.ListConfig[fastly.FOS]{
		TypeSuffix: "_service_logging_fos",
		Label:      "Fastly Object Storage",
		List: func(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.FOS, error) {
			return client.ListFOSes(ctx, &fastly.ListFOSesInput{
				ServiceID:      serviceID,
				ServiceVersion: version,
			})
		},
		Name:     func(a *fastly.FOS) *string { return a.Name },
		SetAttrs: setResourceAttrs,
	})
}

// setResourceAttrs nulls VCL-only fields on Compute so generated config omits them; the schema
// defaults apply on import, matching what Read stores.
func setResourceAttrs(ctx context.Context, result *list.ListResult, s *fastly.FOS, serviceID string, version int, serviceType string) diag.Diagnostics {
	var diags diag.Diagnostics

	id := serviceID + "-" + fmt.Sprintf("%d", version) + "-" + fastly.ToValue(s.Name)

	model := Model{
		NestedModel: FlattenToNestedModel(s),
		ID:          types.StringValue(id),
		Service:     types.StringValue(serviceID),
		Version:     types.Int64Value(int64(version)),
	}
	// The API can report gzip_level alongside a codec, which the validator rejects in generated config.
	if service.StringValue(model.CompressionCodec) != "" {
		model.GzipLevel = types.Int64Null()
	}
	if serviceType == service.TypeCompute {
		model.Format = types.StringNull()
		model.FormatVersion = types.Int64Null()
		model.Placement = types.StringNull()
		model.ResponseCondition = types.StringNull()
	}
	diags.Append(result.Resource.Set(ctx, &model)...)
	return diags
}
