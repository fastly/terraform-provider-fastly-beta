package logginghoneycomb

import (
	"context"
	"fmt"

	"github.com/fastly/terraform-provider-fastly-beta/internal/logging"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v18/fastly"
)

func NewListResource() list.ListResource {
	return logging.NewListResource(logging.ListConfig[fastly.Honeycomb]{
		TypeSuffix: "_service_logging_honeycomb",
		Label:      "Honeycomb",
		List: func(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.Honeycomb, error) {
			return client.ListHoneycombs(ctx, &fastly.ListHoneycombsInput{
				ServiceID:      serviceID,
				ServiceVersion: version,
			})
		},
		Name:     func(a *fastly.Honeycomb) *string { return a.Name },
		SetAttrs: setResourceAttrs,
	})
}

// setResourceAttrs sets the listed resource's attributes on result. serviceType
// mirrors the Compute normalization applied by Create/Read/Import: the API
// still returns its own values for the VCL-only fields on a Compute-attached
// endpoint, but the standalone resource's schema rejects those fields being
// configured on Compute, so they must be reset to their schema defaults here
// too — otherwise this could emit resource data the resource itself can't
// accept.
func setResourceAttrs(ctx context.Context, result *list.ListResult, h *fastly.Honeycomb, serviceID string, version int, serviceType string) diag.Diagnostics {
	var diags diag.Diagnostics

	id := serviceID + "-" + fmt.Sprintf("%d", version) + "-" + fastly.ToValue(h.Name)

	model := Model{
		NestedModel: FlattenToNestedModel(h),
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
