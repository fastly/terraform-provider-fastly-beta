package configstores

import (
	"context"
	"testing"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_configstores", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.Len(t, resp.Schema.Attributes, 2)

	id, ok := resp.Schema.Attributes["id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, id.Computed)

	stores, ok := resp.Schema.Attributes["stores"].(datasourceschema.SetNestedAttribute)
	require.True(t, ok)
	require.True(t, stores.Computed)
	require.Len(t, stores.NestedObject.Attributes, 2)

	storeID, ok := stores.NestedObject.Attributes["id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, storeID.Computed)

	storeName, ok := stores.NestedObject.Attributes["name"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, storeName.Computed)
}

func TestFlattenStores(t *testing.T) {
	stores := []*fastly.ConfigStore{
		{StoreID: "store-b", Name: "beta"},
		nil,
		{StoreID: "store-a", Name: "alpha"},
	}

	setValue, ids, diags := flattenStores(stores)
	require.False(t, diags.HasError(), diags)
	require.ElementsMatch(t, []string{"store-a", "store-b"}, ids)
	require.Len(t, setValue.Elements(), 2)

	got := make(map[string]string, len(setValue.Elements()))
	for _, element := range setValue.Elements() {
		object, ok := element.(types.Object)
		require.True(t, ok)

		attributes := object.Attributes()
		id, ok := attributes["id"].(types.String)
		require.True(t, ok)
		name, ok := attributes["name"].(types.String)
		require.True(t, ok)

		got[id.ValueString()] = name.ValueString()
	}

	require.Equal(t, map[string]string{
		"store-a": "alpha",
		"store-b": "beta",
	}, got)
}

func TestFlattenStoresEmpty(t *testing.T) {
	setValue, ids, diags := flattenStores(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, setValue.Elements())
}
