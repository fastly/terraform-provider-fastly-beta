package botmanagementcategoryresetbots

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v19/fastly"
)

func TestMetadata(t *testing.T) {
	var resp action.MetadataResponse
	NewAction().Metadata(context.Background(), action.MetadataRequest{ProviderTypeName: "fastly"}, &resp)
	assert.Equal(t, "fastly_bot_management_category_reset_bots", resp.TypeName)
}

func TestInvokeSetsEveryBotToInherit(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = w.Write([]byte(`{"meta":{},"data":[{"bot_id":"bot_gptbot","name":"GPTBot","action":{"type":"inherit"}}]}`))
	}))
	t.Cleanup(srv.Close)
	client, err := fastly.NewClientForEndpoint("test-key", srv.URL)
	require.NoError(t, err)

	ctx := context.Background()
	a := &Action{client: client}
	var schemaResp action.SchemaResponse
	a.Schema(ctx, action.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	raw := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}
	require.False(t, raw.Set(ctx, &Model{WorkspaceID: types.StringValue("ws"), CategoryID: types.StringValue("cat_ai_crawler")}).HasError())

	resp := &action.InvokeResponse{}
	a.Invoke(ctx, action.InvokeRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: raw.Raw}}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	assert.Equal(t, http.MethodPatch, gotMethod)
	assert.Equal(t, "/bot-management/v1/workspaces/ws/policy/categories/cat_ai_crawler/bots", gotPath)
	assert.Equal(t, map[string]any{"action": map[string]any{"type": "inherit"}}, gotBody)
}
