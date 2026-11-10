package productenablement

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountProductResources(t *testing.T) {
	tests := []struct {
		constructor func() resource.Resource
		typeName    string
		productID   string
	}{
		{NewAIAcceleratorResource, "fastly_account_product_ai_accelerator", "ai_accelerator"},
		{NewAIRuntimeControlResource, "fastly_account_product_ai_runtime_control", "ai_runtime_control"},
		{NewDomainResearchResource, "fastly_account_product_domain_research", "domain_research"},
		{NewObjectStorageResource, "fastly_account_product_object_storage", "object_storage"},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			r := tt.constructor()
			ctx := context.Background()

			var meta resource.MetadataResponse
			r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "fastly"}, &meta)
			assert.Equal(t, tt.typeName, meta.TypeName)

			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
			require.False(t, schemaResp.Diagnostics.HasError())
			require.False(t, schemaResp.Schema.ValidateImplementation(ctx).HasError())
			assert.NotContains(t, schemaResp.Schema.Attributes, "service_id")
			assert.Contains(t, schemaResp.Schema.Attributes, "enabled")

			importer, ok := r.(resource.ResourceWithImportState)
			require.True(t, ok)

			bad := resource.ImportStateResponse{State: emptyState(ctx, t, r)}
			importer.ImportState(ctx, resource.ImportStateRequest{ID: "not-the-product"}, &bad)
			assert.True(t, bad.Diagnostics.HasError())

			good := resource.ImportStateResponse{State: emptyState(ctx, t, r)}
			importer.ImportState(ctx, resource.ImportStateRequest{ID: tt.productID}, &good)
			assert.False(t, good.Diagnostics.HasError())
		})
	}
}

func emptyState(ctx context.Context, t *testing.T, r resource.Resource) tfsdk.State {
	t.Helper()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	return tfsdk.State{
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
		Schema: schemaResp.Schema,
	}
}
