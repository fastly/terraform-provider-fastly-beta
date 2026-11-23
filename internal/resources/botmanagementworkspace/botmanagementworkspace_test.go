package botmanagementworkspace

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ws "github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces"
)

func TestMetadata(t *testing.T) {
	var resp resource.MetadataResponse
	NewResource().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)
	assert.Equal(t, "fastly_bot_management_workspace", resp.TypeName)
}

func TestBuildUpdateInput(t *testing.T) {
	input := BuildUpdateInput(Model{
		WorkspaceID:    types.StringValue("ws"),
		Name:           types.StringValue("Production"),
		Description:    types.StringValue(""),
		ProtectionMode: types.StringValue("block"),
	})
	assert.Equal(t, "ws", *input.WorkspaceID)
	assert.Equal(t, "Production", *input.Name)
	assert.Equal(t, "", *input.Description)
	assert.Equal(t, "block", *input.ProtectionMode)
}

// Omitted name and description are unknown on create; sending them would
// overwrite the workspace's current values.
func TestBuildUpdateInputOmitsUnknown(t *testing.T) {
	input := BuildUpdateInput(Model{
		WorkspaceID:    types.StringValue("ws"),
		Name:           types.StringUnknown(),
		Description:    types.StringUnknown(),
		ProtectionMode: types.StringValue("log"),
	})
	assert.Nil(t, input.Name)
	assert.Nil(t, input.Description)
	assert.Equal(t, "log", *input.ProtectionMode)
}

func TestFlattenToModel(t *testing.T) {
	m := FlattenToModel(&ws.Workspace{ID: "ws", Name: "Production", ProtectionMode: "block", Services: []string{"svc"}})
	assert.Equal(t, Model{
		ID:             types.StringValue("ws"),
		WorkspaceID:    types.StringValue("ws"),
		Name:           types.StringValue("Production"),
		Description:    types.StringValue(""),
		ProtectionMode: types.StringValue("block"),
	}, m)
}

// The resource has no client here, so any API call would panic.
func TestDeleteOnlyRemovesFromState(t *testing.T) {
	m := FlattenToModel(&ws.Workspace{ID: "ws", Name: "Production", ProtectionMode: "block"})
	resp := &resource.DeleteResponse{}
	(&Resource{}).Delete(context.Background(), resource.DeleteRequest{State: stateForTest(t, &m)}, resp)
	assert.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
}

func TestImportState(t *testing.T) {
	resp := &resource.ImportStateResponse{State: stateForTest(t, nil)}
	(&Resource{}).ImportState(context.Background(), resource.ImportStateRequest{ID: "ws"}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var got Model
	require.False(t, resp.State.Get(context.Background(), &got).HasError())
	assert.Equal(t, "ws", got.WorkspaceID.ValueString())
}

func TestImportStateInvalidID(t *testing.T) {
	for _, id := range []string{"ws/extra", " ws"} {
		resp := &resource.ImportStateResponse{State: stateForTest(t, nil)}
		(&Resource{}).ImportState(context.Background(), resource.ImportStateRequest{ID: id}, resp)
		assert.True(t, resp.Diagnostics.HasError(), id)
	}
}

func stateForTest(t *testing.T, m *Model) tfsdk.State {
	t.Helper()
	var schemaResp resource.SchemaResponse
	(&Resource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	state := tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(context.Background()), nil),
	}
	if m != nil {
		require.False(t, state.Set(context.Background(), m).HasError())
	}
	return state
}
