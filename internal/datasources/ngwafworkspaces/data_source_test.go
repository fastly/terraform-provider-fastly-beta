package ngwafworkspaces

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_ngwaf_workspaces", resp.TypeName)
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

	workspaces, ok := resp.Schema.Attributes["workspaces"].(datasourceschema.SetNestedAttribute)
	require.True(t, ok)
	require.True(t, workspaces.Computed)
	require.Len(t, workspaces.NestedObject.Attributes, 2)

	workspaceID, ok := workspaces.NestedObject.Attributes["id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, workspaceID.Computed)

	workspaceName, ok := workspaces.NestedObject.Attributes["name"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, workspaceName.Computed)
}

func TestFlattenWorkspaces(t *testing.T) {
	workspaces := &ws.Workspaces{
		Data: []ws.Workspace{
			{WorkspaceID: "workspace-b", Name: "beta"},
			{WorkspaceID: "workspace-a", Name: "alpha"},
		},
	}

	setValue, ids, diags := flattenWorkspaces(workspaces)
	require.False(t, diags.HasError(), diags)
	require.ElementsMatch(t, []string{"workspace-a", "workspace-b"}, ids)
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
		"workspace-a": "alpha",
		"workspace-b": "beta",
	}, got)
}

func TestFlattenWorkspacesEmpty(t *testing.T) {
	setValue, ids, diags := flattenWorkspaces(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, setValue.Elements())
}
