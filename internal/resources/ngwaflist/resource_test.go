package ngwaflist

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
)

func TestMetadata(t *testing.T) {
	r := NewWorkspaceResource("ip", "ip", "test resource description")

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_ngwaf_workspace_ip_list", resp.TypeName)
}

func TestSchema(t *testing.T) {
	attrs := Attributes("ip")

	require.Len(t, attrs, 6)

	workspaceID, ok := attrs["workspace_id"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, workspaceID.Required)

	name, ok := attrs["name"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, name.Required)
	assert.Len(t, name.Validators, 1)
	assert.Len(t, name.PlanModifiers, 1)

	description, ok := attrs["description"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, description.Optional)
	assert.True(t, description.Computed)
	assert.Len(t, description.Validators, 1)

	entries, ok := attrs["entries"].(schema.ListAttribute)
	require.True(t, ok)
	assert.True(t, entries.Required)

	referenceID, ok := attrs["reference_id"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, referenceID.Computed)
}

func TestBuildCreateInput(t *testing.T) {
	entries, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"10.0.0.1"})
	require.False(t, diags.HasError(), diags)

	plan := Model{
		WorkspaceID: types.StringValue("workspace-id"),
		Name:        types.StringValue("Example List"),
		Description: types.StringValue("description"),
		Entries:     entries,
	}

	input, diags := BuildCreateInput(context.Background(), "ip", plan)
	require.False(t, diags.HasError(), diags)

	require.NotNil(t, input.Name)
	require.NotNil(t, input.Type)
	require.NotNil(t, input.Description)
	require.NotNil(t, input.Entries)
	require.NotNil(t, input.Scope)
	assert.Equal(t, "Example List", *input.Name)
	assert.Equal(t, "ip", *input.Type)
	assert.Equal(t, "description", *input.Description)
	assert.Equal(t, []string{"10.0.0.1"}, *input.Entries)
	assert.Equal(t, "workspace", string(input.Scope.Type))
	assert.Equal(t, []string{"workspace-id"}, input.Scope.AppliesTo)
}

func TestBuildCreateInputOmitsNullDescription(t *testing.T) {
	entries, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"10.0.0.1"})
	require.False(t, diags.HasError(), diags)

	plan := Model{
		WorkspaceID: types.StringValue("workspace-id"),
		Name:        types.StringValue("Example List"),
		Description: types.StringNull(),
		Entries:     entries,
	}

	input, diags := BuildCreateInput(context.Background(), "ip", plan)
	require.False(t, diags.HasError(), diags)

	require.Nil(t, input.Description)
}

func TestBuildUpdateInput(t *testing.T) {
	entries, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"10.0.0.1", "192.168.1.1"})
	require.False(t, diags.HasError(), diags)

	plan := Model{
		WorkspaceID: types.StringValue("workspace-id"),
		Description: types.StringValue("updated"),
		Entries:     entries,
	}

	input, diags := BuildUpdateInput(context.Background(), "list-id", plan)
	require.False(t, diags.HasError(), diags)

	require.NotNil(t, input.ListID)
	require.NotNil(t, input.Description)
	require.NotNil(t, input.Entries)
	require.NotNil(t, input.Scope)
	assert.Equal(t, "list-id", *input.ListID)
	assert.Equal(t, "updated", *input.Description)
	assert.Equal(t, []string{"10.0.0.1", "192.168.1.1"}, *input.Entries)
	assert.Equal(t, "workspace", string(input.Scope.Type))
	assert.Equal(t, []string{"workspace-id"}, input.Scope.AppliesTo)
}

func TestFlattenToModel(t *testing.T) {
	list := &lists.List{
		ListID:      "list-id",
		Type:        "ip",
		ReferenceID: "workspace.example-list",
		Name:        "Example List",
		Description: "description",
		Entries:     []string{"10.0.0.1"},
		Scope: lists.Scope{
			Type: "workspace",
		},
	}

	model, err := FlattenToModel("ip", "workspace-id", list)
	require.NoError(t, err)

	assert.Equal(t, types.StringValue("list-id"), model.ID)
	assert.Equal(t, types.StringValue("workspace-id"), model.WorkspaceID)
	assert.Equal(t, types.StringValue("Example List"), model.Name)
	assert.Equal(t, types.StringValue("description"), model.Description)
	assert.Equal(t, types.StringValue("workspace.example-list"), model.ReferenceID)
	assert.Equal(t, []string{"10.0.0.1"}, listValueStrings(t, model.Entries))
}

func TestFlattenToModelInvalid(t *testing.T) {
	_, err := FlattenToModel("ip", "workspace-id", nil)
	require.Error(t, err)

	_, err = FlattenToModel("ip", "workspace-id", &lists.List{})
	require.Error(t, err)

	_, err = FlattenToModel("ip", "workspace-id", &lists.List{
		ListID: "list-id",
		Type:   "string",
		Scope:  lists.Scope{Type: "workspace"},
	})
	require.Error(t, err)

	_, err = FlattenToModel("ip", "workspace-id", &lists.List{
		ListID: "list-id",
		Type:   "ip",
		Scope:  lists.Scope{Type: "account"},
	})
	require.Error(t, err)
}

func TestImportStateParserRejectsMalformedIDs(t *testing.T) {
	cases := []string{
		"",
		"workspace-id",
		"/list-id",
		"workspace-id/",
		"workspace-id/list-id/extra",
	}

	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			var resp resource.ImportStateResponse
			ImportState(context.Background(), resource.ImportStateRequest{ID: id}, &resp)
			require.True(t, resp.Diagnostics.HasError())
		})
	}
}

func listValueStrings(t *testing.T, value types.List) []string {
	t.Helper()

	var values []string
	diags := value.ElementsAs(context.Background(), &values, false)
	require.False(t, diags.HasError(), diags)

	return values
}
