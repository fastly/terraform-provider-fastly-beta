package botmanagementworkspace

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

const workspaceJSON = `{"id":"ws","name":"Production","protection_mode":"block","services":["svc"],"created_at":"2026-06-01T19:40:49Z","updated_at":"2026-06-02T19:40:49Z"}`

func TestMetadata(t *testing.T) {
	var resp datasource.MetadataResponse
	NewDataSource().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)
	assert.Equal(t, "fastly_bot_management_workspace", resp.TypeName)
}

func TestReadByServiceID(t *testing.T) {
	var gotQuery string
	d := &DataSource{client: testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bot-management/v1/workspaces", r.URL.Path)
		gotQuery = r.URL.Query().Get("service_id")
		_, _ = w.Write([]byte(`{"meta":{"next_cursor":null},"data":[` + workspaceJSON + `]}`))
	})}

	state, resp := read(t, d, DataSourceModel{ServiceID: types.StringValue("svc"), WorkspaceID: types.StringNull()})
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assert.Equal(t, "svc", gotQuery)
	assertWorkspace(t, state)
	assert.Equal(t, "svc", state.ServiceID.ValueString())
}

func TestReadByWorkspaceID(t *testing.T) {
	d := &DataSource{client: testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bot-management/v1/workspaces/ws", r.URL.Path)
		_, _ = w.Write([]byte(workspaceJSON))
	})}

	state, resp := read(t, d, DataSourceModel{ServiceID: types.StringNull(), WorkspaceID: types.StringValue("ws")})
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assertWorkspace(t, state)
	assert.True(t, state.ServiceID.IsNull())
}

func TestReadByServiceIDErrors(t *testing.T) {
	for name, data := range map[string]string{
		"none":     ``,
		"multiple": workspaceJSON + `,` + workspaceJSON,
	} {
		t.Run(name, func(t *testing.T) {
			d := &DataSource{client: testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"meta":{"next_cursor":null},"data":[` + data + `]}`))
			})}

			_, resp := read(t, d, DataSourceModel{ServiceID: types.StringValue("svc"), WorkspaceID: types.StringNull()})
			assert.True(t, resp.Diagnostics.HasError())
		})
	}
}

func TestReadByWorkspaceIDNotFound(t *testing.T) {
	d := &DataSource{client: testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"title":"Not found","status":404}`))
	})}

	_, resp := read(t, d, DataSourceModel{ServiceID: types.StringNull(), WorkspaceID: types.StringValue("missing")})
	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Bot Management workspace not found", resp.Diagnostics[0].Summary())
}

func assertWorkspace(t *testing.T, state DataSourceModel) {
	t.Helper()
	assert.Equal(t, "ws", state.ID.ValueString())
	assert.Equal(t, "ws", state.WorkspaceID.ValueString())
	assert.Equal(t, "Production", state.Name.ValueString())
	assert.Equal(t, "", state.Description.ValueString())
	assert.Equal(t, "block", state.ProtectionMode.ValueString())
	assert.Equal(t, "2026-06-01T19:40:49Z", state.CreatedAt.ValueString())
	assert.Equal(t, "2026-06-02T19:40:49Z", state.UpdatedAt.ValueString())

	var services []string
	require.False(t, state.Services.ElementsAs(context.Background(), &services, false).HasError())
	assert.Equal(t, []string{"svc"}, services)
}

func read(t *testing.T, d *DataSource, config DataSourceModel) (DataSourceModel, *datasource.ReadResponse) {
	t.Helper()
	ctx := context.Background()

	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	config.ID = types.StringNull()
	config.Name = types.StringNull()
	config.Description = types.StringNull()
	config.ProtectionMode = types.StringNull()
	config.Services = types.ListNull(types.StringType)
	config.CreatedAt = types.StringNull()
	config.UpdatedAt = types.StringNull()

	raw := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}
	require.False(t, raw.Set(ctx, &config).HasError())

	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: raw.Raw}}, resp)

	var state DataSourceModel
	if !resp.Diagnostics.HasError() {
		require.False(t, resp.State.Get(ctx, &state).HasError())
	}
	return state, resp
}

func testClient(t *testing.T, handler http.HandlerFunc) *fastly.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client, err := fastly.NewClientForEndpoint("test-key", srv.URL)
	require.NoError(t, err)
	return client
}
