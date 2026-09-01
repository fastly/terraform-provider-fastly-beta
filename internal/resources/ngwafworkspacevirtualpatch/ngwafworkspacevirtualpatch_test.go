package ngwafworkspacevirtualpatch

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vp "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/virtualpatches"
)

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)

	assert.Equal(t, "fastly_ngwaf_workspace_virtual_patch", resp.TypeName)
}

func TestSchema(t *testing.T) {
	attrs := ResourceAttributes()

	require.Len(t, attrs, 6)

	workspaceID, ok := attrs["workspace_id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, workspaceID.Required)

	virtualPatchID, ok := attrs["virtual_patch_id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, virtualPatchID.Required)

	mode, ok := attrs["mode"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, mode.Required)

	description, ok := attrs["description"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, description.Computed)
}

func TestBuildGetInput(t *testing.T) {
	input := BuildGetInput("workspace-id", "CVE-2017-5638")

	assert.Equal(t, "workspace-id", *input.WorkspaceID)
	assert.Equal(t, "CVE-2017-5638", *input.VirtualPatchID)
}

func TestBuildUpdateInput(t *testing.T) {
	plan := Model{
		Mode:    types.StringValue("block"),
		Enabled: types.BoolValue(true),
	}

	input := BuildUpdateInput("workspace-id", "CVE-2017-5638", plan)

	assert.Equal(t, "workspace-id", *input.WorkspaceID)
	assert.Equal(t, "CVE-2017-5638", *input.VirtualPatchID)
	assert.Equal(t, "block", *input.Mode)
	assert.True(t, *input.Enabled)
}

func TestBuildDisableInput(t *testing.T) {
	input := BuildDisableInput("workspace-id", "CVE-2017-5638", "log")

	assert.Equal(t, "workspace-id", *input.WorkspaceID)
	assert.Equal(t, "CVE-2017-5638", *input.VirtualPatchID)
	assert.Equal(t, "log", *input.Mode)
	assert.False(t, *input.Enabled)
}

func TestFlattenToModel(t *testing.T) {
	virtualPatch := &vp.VirtualPatch{
		ID:          "CVE-2017-5638",
		Description: "Apache Struts virtual patch",
		Mode:        "block",
		Enabled:     true,
	}

	m, err := FlattenToModel("workspace-id", virtualPatch)
	require.NoError(t, err)

	assert.Equal(t, types.StringValue("CVE-2017-5638"), m.ID)
	assert.Equal(t, types.StringValue("workspace-id"), m.WorkspaceID)
	assert.Equal(t, types.StringValue("CVE-2017-5638"), m.VirtualPatchID)
	assert.Equal(t, types.StringValue("block"), m.Mode)
	assert.Equal(t, types.BoolValue(true), m.Enabled)
	assert.Equal(t, types.StringValue("Apache Struts virtual patch"), m.Description)
}

func TestFlattenToModelInvalid(t *testing.T) {
	_, err := FlattenToModel("workspace-id", nil)
	require.Error(t, err)

	_, err = FlattenToModel("workspace-id", &vp.VirtualPatch{})
	require.Error(t, err)
}

func TestImportState(t *testing.T) {
	r := &Resource{}

	req := resource.ImportStateRequest{ID: "workspace-id/CVE-2017-5638"}
	resp := &resource.ImportStateResponse{State: importStateForTest(t)}

	r.ImportState(context.Background(), req, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var got Model
	diags := resp.State.Get(context.Background(), &got)
	require.False(t, diags.HasError(), diags)
	assert.Equal(t, "workspace-id", got.WorkspaceID.ValueString())
	assert.Equal(t, "CVE-2017-5638", got.ID.ValueString())
	assert.Equal(t, "CVE-2017-5638", got.VirtualPatchID.ValueString())
}

func TestImportStateInvalidID(t *testing.T) {
	cases := []string{
		"not-a-valid-id",
		"workspace-id/",
		"/CVE-2017-5638",
		"workspace-id/CVE-2017-5638/extra",
	}

	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			r := &Resource{}

			req := resource.ImportStateRequest{ID: id}
			resp := &resource.ImportStateResponse{State: importStateForTest(t)}

			r.ImportState(context.Background(), req, resp)
			assert.True(t, resp.Diagnostics.HasError())
		})
	}
}

func importStateForTest(t *testing.T) tfsdk.State {
	t.Helper()

	var res Resource
	var schemaResp resource.SchemaResponse
	res.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	tfType := schemaResp.Schema.Type().TerraformType(context.Background())
	return tfsdk.State{
		Raw:    tftypes.NewValue(tfType, nil),
		Schema: schemaResp.Schema,
	}
}
