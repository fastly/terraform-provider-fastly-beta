package loggingkinesis

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
	return logging.NewListResource(logging.ListConfig[fastly.Kinesis]{
		TypeSuffix: "_service_logging_kinesis",
		Label:      "Kinesis",
		List: func(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.Kinesis, error) {
			return client.ListKinesis(ctx, &fastly.ListKinesisInput{
				ServiceID:      serviceID,
				ServiceVersion: version,
			})
		},
		Name:     func(a *fastly.Kinesis) *string { return a.Name },
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
func setResourceAttrs(ctx context.Context, result *list.ListResult, k *fastly.Kinesis, serviceID string, version int, serviceType string) diag.Diagnostics {
	var diags diag.Diagnostics

	id := serviceID + "-" + fmt.Sprintf("%d", version) + "-" + fastly.ToValue(k.Name)

	model := Model{
		NestedModel: FlattenToNestedModel(k),
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
