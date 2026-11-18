package botmanagementcategory

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

func TestMetadata(t *testing.T) {
	var resp resource.MetadataResponse
	NewResource().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)
	assert.Equal(t, "fastly_bot_management_category", resp.TypeName)
}

func TestActionValidator(t *testing.T) {
	attr, ok := ResourceAttributes()["action"].(schema.StringAttribute)
	require.True(t, ok)

	for value, wantErr := range map[string]bool{"allow": false, "block": false, "challenge": false, "inherit": true} {
		var resp validator.StringResponse
		for _, v := range attr.Validators {
			v.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(value)}, &resp)
		}
		assert.Equal(t, wantErr, resp.Diagnostics.HasError(), value)
	}
}

func TestBuildUpdateInput(t *testing.T) {
	input := BuildUpdateInput(testModel())
	assert.Equal(t, "ws", *input.WorkspaceID)
	assert.Equal(t, "cat_ai_crawler", *input.CategoryID)
	assert.Equal(t, "block", input.Action.Type)
}

func TestFlattenToModel(t *testing.T) {
	m := FlattenToModel("ws", &policy.Category{
		CategoryID:  "cat_ai_crawler",
		Name:        "AI Crawlers",
		Description: "Bots that crawl content for AI training datasets.",
		Action:      policy.Action{Type: "block"},
	})
	assert.Equal(t, testModel(), m)
}

// The resource has no client here, so any API call would panic.
func TestDeleteOnlyRemovesFromState(t *testing.T) {
	resp := &resource.DeleteResponse{}
	(&Resource{}).Delete(context.Background(), resource.DeleteRequest{State: stateForTest(t, new(testModel()))}, resp)
	assert.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
}

func TestImportState(t *testing.T) {
	resp := &resource.ImportStateResponse{State: stateForTest(t, nil)}
	(&Resource{}).ImportState(context.Background(), resource.ImportStateRequest{ID: "ws/cat_ai_crawler"}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var got Model
	require.False(t, resp.State.Get(context.Background(), &got).HasError())
	assert.Equal(t, "ws", got.WorkspaceID.ValueString())
	assert.Equal(t, "cat_ai_crawler", got.CategoryID.ValueString())
}

func TestImportStateInvalidID(t *testing.T) {
	for _, id := range []string{"ws", "ws/", "/cat_ai_crawler", "ws/cat_ai_crawler/extra"} {
		resp := &resource.ImportStateResponse{State: stateForTest(t, nil)}
		(&Resource{}).ImportState(context.Background(), resource.ImportStateRequest{ID: id}, resp)
		assert.True(t, resp.Diagnostics.HasError(), id)
	}
}

func testModel() Model {
	return Model{
		ID:          types.StringValue("ws/cat_ai_crawler"),
		WorkspaceID: types.StringValue("ws"),
		CategoryID:  types.StringValue("cat_ai_crawler"),
		Action:      types.StringValue("block"),
		Name:        types.StringValue("AI Crawlers"),
		Description: types.StringValue("Bots that crawl content for AI training datasets."),
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
