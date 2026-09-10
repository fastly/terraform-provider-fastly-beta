package apisecuritydiscoveredoperations

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_api_security_discovered_operations", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.Len(t, resp.Schema.Attributes, 8)

	serviceID, ok := resp.Schema.Attributes["service_id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, serviceID.Required)

	operationsAttr, ok := resp.Schema.Attributes["operations"].(datasourceschema.ListNestedAttribute)
	require.True(t, ok)
	require.True(t, operationsAttr.Computed)
	require.Len(t, operationsAttr.NestedObject.Attributes, 8)
}

func TestFlattenDiscoveredOperations(t *testing.T) {
	remote := []operations.DiscoveredOperation{
		{
			ID:     "op-b",
			Method: "GET",
			Domain: "api.example.com",
			Path:   "/v1/things",
			Status: "DISCOVERED",
			RPS:    1.5,
		},
		{
			ID:         "op-a",
			Method:     "POST",
			Domain:     "api.example.com",
			Path:       "/v1/other",
			UpdatedAt:  "2026-01-02T03:04:05Z",
			LastSeenAt: "2026-01-02T03:04:05Z",
		},
	}

	listValue, diags := flattenDiscoveredOperations(remote)
	require.False(t, diags.HasError(), diags)
	require.Len(t, listValue.Elements(), 2)

	first, ok := listValue.Elements()[0].(types.Object)
	require.True(t, ok)
	attributes := first.Attributes()

	require.Equal(t, "op-b", attributes["id"].(types.String).ValueString())
	require.Equal(t, "GET", attributes["method"].(types.String).ValueString())
	require.Equal(t, 1.5, attributes["rps"].(types.Float64).ValueFloat64())
	require.Equal(t, "DISCOVERED", attributes["status"].(types.String).ValueString())
	require.True(t, attributes["updated_at"].(types.String).IsNull())

	second, ok := listValue.Elements()[1].(types.Object)
	require.True(t, ok)
	attributes = second.Attributes()

	require.True(t, attributes["status"].(types.String).IsNull())
	require.Equal(t, "2026-01-02T03:04:05Z", attributes["updated_at"].(types.String).ValueString())
	require.Equal(t, "2026-01-02T03:04:05Z", attributes["last_seen_at"].(types.String).ValueString())
}

func TestFlattenDiscoveredOperationsEmpty(t *testing.T) {
	listValue, diags := flattenDiscoveredOperations(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, listValue.Elements())
}

func TestIDsOf(t *testing.T) {
	remote := []operations.DiscoveredOperation{
		{ID: "op-a"},
		{ID: "op-b"},
	}

	require.Equal(t, []string{"op-a", "op-b"}, idsOf(remote))
}
