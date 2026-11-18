package botmanagementcategories

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
	assert.Equal(t, "fastly_bot_management_categories", resp.TypeName)
}

func TestRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bot-management/v1/workspaces/ws/policy/categories", r.URL.Path)
		_, _ = w.Write([]byte(`{"meta":{"next_cursor":null},"data":[{"category_id":"cat_ai_crawler","name":"AI Crawlers","description":"Bots that crawl content for AI training datasets.","action":{"type":"block"}}]}`))
	}))
	t.Cleanup(srv.Close)
	client, err := fastly.NewClientForEndpoint("test-key", srv.URL)
	require.NoError(t, err)
	d := &DataSource{client: client}

	ctx := context.Background()
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	empty := func() tfsdk.State {
		return tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}
	}
	raw := empty()
	require.False(t, raw.Set(ctx, &DataSourceModel{ID: types.StringNull(), WorkspaceID: types.StringValue("ws")}).HasError())

	resp := &datasource.ReadResponse{State: empty()}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: raw.Raw}}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var state DataSourceModel
	require.False(t, resp.State.Get(ctx, &state).HasError())
	require.Len(t, state.Categories, 1)
	assert.Equal(t, CategoryModel{
		CategoryID:  types.StringValue("cat_ai_crawler"),
		Name:        types.StringValue("AI Crawlers"),
		Description: types.StringValue("Bots that crawl content for AI training datasets."),
		Action:      types.StringValue("block"),
	}, state.Categories[0])
	assert.NotEmpty(t, state.ID.ValueString())
}
