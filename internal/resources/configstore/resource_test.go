package configstore

import (
	"context"
	"testing"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_configstore", resp.TypeName)
}

func TestSchema(t *testing.T) {
	r := NewResource()

	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.Len(t, resp.Schema.Attributes, 2)

	id, ok := resp.Schema.Attributes["id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, id.Computed)
	require.False(t, id.Required)
	require.NotEmpty(t, id.PlanModifiers)

	name, ok := resp.Schema.Attributes["name"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, name.Required)
	require.False(t, name.Computed)
	require.NotEmpty(t, name.Validators)
	require.Empty(t, name.PlanModifiers, "Config Store names are mutable and must not require replacement")

}

func TestFlattenNilStoreLeavesModelUntouched(t *testing.T) {
	model := Model{
		ID:   types.StringValue("existing-id"),
		Name: types.StringValue("existing-name"),
	}

	flatten(&model, nil)

	require.Equal(t, types.StringValue("existing-id"), model.ID)
	require.Equal(t, types.StringValue("existing-name"), model.Name)
}

func TestFlattenConfigStore(t *testing.T) {
	model := Model{}

	flatten(&model, &fastly.ConfigStore{
		StoreID: "store-id",
		Name:    "example-store",
	})

	require.Equal(t, types.StringValue("store-id"), model.ID)
	require.Equal(t, types.StringValue("example-store"), model.Name)
}
