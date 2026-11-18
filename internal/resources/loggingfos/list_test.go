package loggingfos

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"

	fastly "github.com/fastly/go-fastly/v19/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

// TestSetResourceAttrs_computeResetsVCLOnlyFields checks that the list output nulls the API's VCL-only
// values for Compute endpoints.
func TestSetResourceAttrs_computeResetsVCLOnlyFields(t *testing.T) {
	ctx := context.Background()

	s := &fastly.FOS{
		ServiceID:         new("service-id"),
		ServiceVersion:    new(1),
		Name:              new("logger"),
		BucketName:        new("bucket"),
		Format:            new("api-assigned-compute-format"),
		FormatVersion:     new(2),
		Placement:         new("none"),
		ResponseCondition: new("some-condition"),
	}

	tests := []struct {
		name              string
		serviceType       string
		wantFormat        types.String
		wantFormatVersion types.Int64
		wantPlacement     types.String
		wantRespCondition types.String
	}{
		{
			name:              "vcl service keeps the API's values",
			serviceType:       service.TypeVCL,
			wantFormat:        types.StringValue("api-assigned-compute-format"),
			wantFormatVersion: types.Int64Value(2),
			wantPlacement:     types.StringValue("none"),
			wantRespCondition: types.StringValue("some-condition"),
		},
		{
			name:              "compute service nulls VCL-only fields so generated config omits them",
			serviceType:       service.TypeCompute,
			wantFormat:        types.StringNull(),
			wantFormatVersion: types.Int64Null(),
			wantPlacement:     types.StringNull(),
			wantRespCondition: types.StringNull(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTestListResult(ctx)

			diags := setResourceAttrs(ctx, &result, s, "service-id", 1, tt.serviceType)
			require.False(t, diags.HasError(), diags)

			var got Model
			require.False(t, result.Resource.Get(ctx, &got).HasError())

			require.Equal(t, tt.wantFormat, got.Format)
			require.Equal(t, tt.wantFormatVersion, got.FormatVersion)
			require.Equal(t, tt.wantPlacement, got.Placement)
			require.Equal(t, tt.wantRespCondition, got.ResponseCondition)
		})
	}
}

// TestSetResourceAttrs_codecDropsGzipLevel checks that an endpoint the API reports with both a codec
// and gzip_level yields a null gzip_level, so generated config passes the conflict validator.
func TestSetResourceAttrs_codecDropsGzipLevel(t *testing.T) {
	ctx := context.Background()

	for name, tt := range map[string]struct {
		codec *string
		want  types.Int64
	}{
		"codec set": {codec: new("zstd"), want: types.Int64Null()},
		"no codec":  {codec: nil, want: types.Int64Value(3)},
	} {
		t.Run(name, func(t *testing.T) {
			s := &fastly.FOS{
				Name:             new("logger"),
				BucketName:       new("bucket"),
				GzipLevel:        new(3),
				CompressionCodec: tt.codec,
			}
			result := buildTestListResult(ctx)
			require.False(t, setResourceAttrs(ctx, &result, s, "service-id", 1, service.TypeVCL).HasError())

			var got Model
			require.False(t, result.Resource.Get(ctx, &got).HasError())
			require.Equal(t, tt.want, got.GzipLevel)
		})
	}
}

// buildTestListResult builds a list.ListResult backed by the standalone resource schema.
func buildTestListResult(ctx context.Context) list.ListResult {
	s := schema.Schema{Attributes: ResourceAttributes()}
	return list.ListResult{
		Resource: &tfsdk.Resource{
			Raw:    tftypes.NewValue(s.Type().TerraformType(ctx), nil),
			Schema: s,
		},
	}
}
