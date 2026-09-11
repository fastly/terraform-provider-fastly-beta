package apisecurityoperationtags

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

	require.Equal(t, "fastly_api_security_operation_tags", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.Len(t, resp.Schema.Attributes, 4)

	serviceID, ok := resp.Schema.Attributes["service_id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, serviceID.Required)

	tagsAttr, ok := resp.Schema.Attributes["tags"].(datasourceschema.ListNestedAttribute)
	require.True(t, ok)
	require.True(t, tagsAttr.Computed)
	require.Len(t, tagsAttr.NestedObject.Attributes, 6)
}

func TestFlattenTags(t *testing.T) {
	remote := []operations.OperationTag{
		{ID: "tag-b", Name: "beta", Count: 2},
		{
			ID:          "tag-a",
			Name:        "alpha",
			Description: "first tag",
			CreatedAt:   "2026-01-01T00:00:00Z",
			UpdatedAt:   "2026-01-02T03:04:05Z",
		},
	}

	listValue, diags := flattenTags(remote)
	require.False(t, diags.HasError(), diags)
	require.Len(t, listValue.Elements(), 2)

	// remote is deliberately given out of ID order; flattenTags must sort by
	// ID regardless of API-returned order.
	first, ok := listValue.Elements()[0].(types.Object)
	require.True(t, ok)
	attributes := first.Attributes()

	require.Equal(t, "tag-a", attributes["id"].(types.String).ValueString())
	require.Equal(t, "first tag", attributes["description"].(types.String).ValueString())
	require.Equal(t, "2026-01-01T00:00:00Z", attributes["created_at"].(types.String).ValueString())
	require.Equal(t, int64(0), attributes["operation_count"].(types.Int64).ValueInt64())

	second, ok := listValue.Elements()[1].(types.Object)
	require.True(t, ok)
	attributes = second.Attributes()

	require.Equal(t, "tag-b", attributes["id"].(types.String).ValueString())
	require.Equal(t, "beta", attributes["name"].(types.String).ValueString())
	require.True(t, attributes["description"].(types.String).IsNull())
	require.Equal(t, int64(2), attributes["operation_count"].(types.Int64).ValueInt64())
}

func TestFlattenTagsEmpty(t *testing.T) {
	listValue, diags := flattenTags(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, listValue.Elements())
}

func TestIDsOf(t *testing.T) {
	remote := []operations.OperationTag{
		{ID: "tag-a"},
		{ID: "tag-b"},
	}

	require.Equal(t, []string{"tag-a", "tag-b"}, idsOf(remote))
}
