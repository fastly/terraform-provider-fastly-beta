package botmanagementbots

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v19/fastly"
)

func TestMetadata(t *testing.T) {
	var resp datasource.MetadataResponse
	NewDataSource().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)
	assert.Equal(t, "fastly_bot_management_bots", resp.TypeName)
}

func TestReadWorkspaceWide(t *testing.T) {
	d := &DataSource{client: testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bot-management/v1/workspaces/ws/policy/bots", r.URL.Path)
		_, _ = w.Write([]byte(`{"meta":{"next_cursor":null},"data":[{"bot_id":"bot_gptbot","name":"GPTBot","category_id":"cat_ai_crawler","action":{"type":"inherit"}}]}`))
	})}

	state := read(t, d, types.StringNull())
	require.Len(t, state.Bots, 1)
	assert.Equal(t, "bot_gptbot", state.Bots[0].BotID.ValueString())
	assert.Equal(t, "cat_ai_crawler", state.Bots[0].CategoryID.ValueString())
	assert.Equal(t, "inherit", state.Bots[0].Action.ValueString())
}

// The category-scoped endpoint omits category_id from each bot, so it's
// filled in from the filter.
func TestReadByCategoryFillsCategoryID(t *testing.T) {
	d := &DataSource{client: testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bot-management/v1/workspaces/ws/policy/categories/cat_ai_crawler/bots", r.URL.Path)
		_, _ = w.Write([]byte(`{"meta":{"next_cursor":null},"data":[{"bot_id":"bot_gptbot","name":"GPTBot","action":{"type":"allow"}}]}`))
	})}

	state := read(t, d, types.StringValue("cat_ai_crawler"))
	require.Len(t, state.Bots, 1)
	assert.Equal(t, "cat_ai_crawler", state.Bots[0].CategoryID.ValueString())
	assert.Equal(t, "allow", state.Bots[0].Action.ValueString())
}

func read(t *testing.T, d *DataSource, categoryID types.String) DataSourceModel {
	t.Helper()
	ctx := context.Background()

	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	empty := func() tfsdk.State {
		return tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}
	}
	raw := empty()
	require.False(t, raw.Set(ctx, &DataSourceModel{
		ID:          types.StringNull(),
		WorkspaceID: types.StringValue("ws"),
		CategoryID:  categoryID,
	}).HasError())

	resp := &datasource.ReadResponse{State: empty()}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: raw.Raw}}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var state DataSourceModel
	require.False(t, resp.State.Get(ctx, &state).HasError())
	return state
}

func testClient(t *testing.T, handler http.HandlerFunc) *fastly.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client, err := fastly.NewClientForEndpoint("test-key", srv.URL)
	require.NoError(t, err)
	return client
}
