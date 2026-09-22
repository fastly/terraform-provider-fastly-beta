package dictionary

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

	d := &fastly.Dictionary{
		ServiceID:      new("service-id"),
		ServiceVersion: new(3),
		Name:           new("my_dictionary"),
		DictionaryID:   new("dict_abc123"),
		WriteOnly:      new(true),
	}

	result := buildTestListResult(ctx)

	diags := setResourceAttrs(ctx, &result, d, "service-id", 3)
	require.False(t, diags.HasError(), diags)

	var got Model
	require.False(t, result.Resource.Get(ctx, &got).HasError())

	require.Equal(t, types.StringValue("service-id-3-my_dictionary"), got.ID)
	require.Equal(t, types.StringValue("service-id"), got.Service)
	require.Equal(t, types.Int64Value(3), got.Version)
	require.Equal(t, types.StringValue("my_dictionary"), got.Name)
	require.Equal(t, types.StringValue("dict_abc123"), got.DictionaryID)
	require.Equal(t, types.BoolValue(true), got.WriteOnly)
	// ForceDestroy is configuration-only and never returned by the API, so
	// setResourceAttrs (via ops.ToModel) always reports the schema default here.
	require.Equal(t, types.BoolValue(DefaultForceDestroy), got.ForceDestroy)
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
