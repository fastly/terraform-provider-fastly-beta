package ngwaflist

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
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

func TestAccountMetadata(t *testing.T) {
	tests := []struct {
		listType   string
		typeSuffix string
		want       string
	}{
		{listType: "country", typeSuffix: "country", want: "fastly_ngwaf_country_list"},
		{listType: "ip", typeSuffix: "ip", want: "fastly_ngwaf_ip_list"},
		{listType: "signal", typeSuffix: "signal", want: "fastly_ngwaf_signal_list"},
		{listType: "string", typeSuffix: "string", want: "fastly_ngwaf_string_list"},
		{listType: "wildcard", typeSuffix: "wildcard", want: "fastly_ngwaf_wildcard_list"},
	}

	for _, tt := range tests {
		t.Run(tt.listType, func(t *testing.T) {
			r := NewAccountResource(tt.listType, tt.typeSuffix, "test")

			var resp resource.MetadataResponse
			r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)

			require.Equal(t, tt.want, resp.TypeName)
		})
	}
}

func TestAccountSchema(t *testing.T) {
	attrs := AccountAttributes("ip")
	require.Len(t, attrs, 5)

	_, hasWorkspaceID := attrs["workspace_id"]
	assert.False(t, hasWorkspaceID)

	_, hasType := attrs["type"]
	assert.False(t, hasType)

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

func TestBuildAccountCreateInput(t *testing.T) {
	entries := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("10.0.0.1"),
	})

	plan := AccountModel{
		Name:        types.StringValue("Example IP List"),
		Description: types.StringValue("description"),
		Entries:     entries,
	}

	input, diags := BuildAccountCreateInput(context.Background(), "ip", plan)
	require.False(t, diags.HasError(), diags)

	require.NotNil(t, input.Name)
	require.NotNil(t, input.Type)
	require.NotNil(t, input.Description)
	require.NotNil(t, input.Entries)
	require.NotNil(t, input.Scope)
	assert.Equal(t, "Example IP List", *input.Name)
	assert.Equal(t, "ip", *input.Type)
	assert.Equal(t, "description", *input.Description)
	assert.Equal(t, []string{"10.0.0.1"}, *input.Entries)
	assert.Equal(t, scope.ScopeTypeAccount, input.Scope.Type)
	assert.Empty(t, input.Scope.AppliesTo)
}

func TestBuildAccountCreateInputOmitsNullDescription(t *testing.T) {
	entries := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("10.0.0.1"),
	})

	plan := AccountModel{
		Name:        types.StringValue("Example IP List"),
		Description: types.StringNull(),
		Entries:     entries,
	}

	input, diags := BuildAccountCreateInput(context.Background(), "ip", plan)
	require.False(t, diags.HasError(), diags)
	require.Nil(t, input.Description)
}

func TestBuildAccountUpdateInput(t *testing.T) {
	entries := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("10.0.0.1"),
		types.StringValue("192.168.1.1"),
	})

	plan := AccountModel{
		Description: types.StringValue("updated"),
		Entries:     entries,
	}

	input, diags := BuildAccountUpdateInput(context.Background(), "list-id", plan)
	require.False(t, diags.HasError(), diags)

	require.NotNil(t, input.ListID)
	require.NotNil(t, input.Description)
	require.NotNil(t, input.Entries)
	require.NotNil(t, input.Scope)
	assert.Equal(t, "list-id", *input.ListID)
	assert.Equal(t, "updated", *input.Description)
	assert.Equal(t, []string{"10.0.0.1", "192.168.1.1"}, *input.Entries)
	assert.Equal(t, scope.ScopeTypeAccount, input.Scope.Type)
	assert.Empty(t, input.Scope.AppliesTo)
}

func TestBuildAccountGetAndDeleteInputs(t *testing.T) {
	getInput := BuildAccountGetInput("list-id")
	require.NotNil(t, getInput.ListID)
	require.NotNil(t, getInput.Scope)
	assert.Equal(t, "list-id", *getInput.ListID)
	assert.Equal(t, scope.ScopeTypeAccount, getInput.Scope.Type)
	assert.Empty(t, getInput.Scope.AppliesTo)

	deleteInput := BuildAccountDeleteInput("list-id")
	require.NotNil(t, deleteInput.ListID)
	require.NotNil(t, deleteInput.Scope)
	assert.Equal(t, "list-id", *deleteInput.ListID)
	assert.Equal(t, scope.ScopeTypeAccount, deleteInput.Scope.Type)
	assert.Empty(t, deleteInput.Scope.AppliesTo)
}

func TestFlattenAccountToModel(t *testing.T) {
	list := &lists.List{
		ListID:      "list-id",
		Type:        "ip",
		ReferenceID: "account.example-list",
		Name:        "Example IP List",
		Description: "description",
		Entries:     []string{"10.0.0.1"},
		Scope: lists.Scope{
			Type: "account",
		},
	}

	model, err := FlattenAccountToModel("ip", list)
	require.NoError(t, err)

	assert.Equal(t, types.StringValue("list-id"), model.ID)
	assert.Equal(t, types.StringValue("Example IP List"), model.Name)
	assert.Equal(t, types.StringValue("description"), model.Description)
	assert.Equal(t, types.StringValue("account.example-list"), model.ReferenceID)
	assert.Equal(t, []string{"10.0.0.1"}, listValueStrings(t, model.Entries))
}

func TestFlattenAccountToModelInvalid(t *testing.T) {
	_, err := FlattenAccountToModel("ip", nil)
	require.Error(t, err)

	_, err = FlattenAccountToModel("ip", &lists.List{})
	require.Error(t, err)

	_, err = FlattenAccountToModel("ip", &lists.List{
		ListID: "list-id",
		Type:   "string",
		Scope:  lists.Scope{Type: "account"},
	})
	require.Error(t, err)

	_, err = FlattenAccountToModel("ip", &lists.List{
		ListID: "list-id",
		Type:   "ip",
		Scope:  lists.Scope{Type: "workspace"},
	})
	require.Error(t, err)
}

func TestAccountImportState(t *testing.T) {
	r := NewAccountResource("ip", "ip", "test").(*AccountResource)
	req := resource.ImportStateRequest{ID: "list-id"}
	resp := &resource.ImportStateResponse{State: accountImportStateForTest(t, r)}

	r.ImportState(context.Background(), req, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var gotID types.String
	diags := resp.State.GetAttribute(context.Background(), path.Root("id"), &gotID)
	require.False(t, diags.HasError(), diags)
	assert.Equal(t, "list-id", gotID.ValueString())
}

func accountImportStateForTest(t *testing.T, r *AccountResource) tfsdk.State {
	t.Helper()

	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	tfType := schemaResp.Schema.Type().TerraformType(context.Background())
	return tfsdk.State{
		Raw:    tftypes.NewValue(tfType, nil),
		Schema: schemaResp.Schema,
	}
}
