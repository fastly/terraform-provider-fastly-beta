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

// setResourceAttrs mirrors Create/Read: VCL-only fields are reset on Compute so the list output
// stays valid for the standalone resource's schema.
func setResourceAttrs(ctx context.Context, result *list.ListResult, s *fastly.FOS, serviceID string, version int, serviceType string) diag.Diagnostics {
	var diags diag.Diagnostics

	id := serviceID + "-" + fmt.Sprintf("%d", version) + "-" + fastly.ToValue(s.Name)

	model := Model{
		NestedModel: FlattenToNestedModel(s),
		ID:          types.StringValue(id),
		Service:     types.StringValue(serviceID),
		Version:     types.Int64Value(int64(version)),
	}
	if serviceType == service.TypeCompute {
		ResetVCLOnlyToDefaults(&model.NestedModel)
	}
	diags.Append(result.Resource.Set(ctx, &model)...)
	return diags
}
