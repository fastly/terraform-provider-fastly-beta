package resourcelink

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func TestSetResourceAttrs(t *testing.T) {
	ctx := context.Background()

	link := &fastly.Resource{
		Name:       new("my_store"),
		ResourceID: new("kv_abc123"),
		LinkID:     new("link_xyz789"),
	}

	result := buildTestListResult(ctx)

	diags := setResourceAttrs(ctx, &result, link, "service-id", 4)
	require.False(t, diags.HasError(), diags)

	var got Model
	require.False(t, result.Resource.Get(ctx, &got).HasError())

	require.Equal(t, types.StringValue("service-id-4-link_xyz789"), got.ID)
	require.Equal(t, types.StringValue("service-id"), got.Service)
	require.Equal(t, types.Int64Value(4), got.Version)
	require.Equal(t, types.StringValue("my_store"), got.Name)
	require.Equal(t, types.StringValue("kv_abc123"), got.ResourceID)
	require.Equal(t, types.StringValue("link_xyz789"), got.LinkID)
}

// buildTestListResult builds a list.ListResult backed by the standalone resource's own schema,
// without requiring a full list.ListRequest.
func buildTestListResult(ctx context.Context) list.ListResult {
	s := schema.Schema{Attributes: ResourceAttributes()}
	return list.ListResult{
		Resource: &tfsdk.Resource{
			Raw:    tftypes.NewValue(s.Type().TerraformType(ctx), nil),
			Schema: s,
		},
	}
}
