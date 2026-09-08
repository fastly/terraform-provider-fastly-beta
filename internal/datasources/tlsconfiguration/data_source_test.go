package tlsconfiguration

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_tls_configuration", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	id, ok := resp.Schema.Attributes["id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, id.Optional)
	require.True(t, id.Computed)

	dnsRecords, ok := resp.Schema.Attributes["dns_records"].(datasourceschema.SetNestedAttribute)
	require.True(t, ok)
	require.True(t, dnsRecords.Computed)
	require.Len(t, dnsRecords.NestedObject.Attributes, 3)
}

func TestContainsAll(t *testing.T) {
	require.True(t, containsAll([]string{"TLSv1.2", "TLSv1.3"}, []string{"TLSv1.3"}))
	require.True(t, containsAll([]string{"TLSv1.2", "TLSv1.3"}, nil))
	require.False(t, containsAll([]string{"TLSv1.2"}, []string{"TLSv1.3"}))
}

func newState(t *testing.T) DataSourceModel {
	t.Helper()

	tlsProtocols, diags := types.SetValueFrom(context.Background(), types.StringType, []string(nil))
	require.False(t, diags.HasError(), diags)
	httpProtocols, diags := types.SetValueFrom(context.Background(), types.StringType, []string(nil))
	require.False(t, diags.HasError(), diags)

	return DataSourceModel{
		ID:            types.StringNull(),
		Name:          types.StringNull(),
		TLSProtocols:  types.SetNull(tlsProtocols.ElementType(context.Background())),
		HTTPProtocols: types.SetNull(httpProtocols.ElementType(context.Background())),
		TLSService:    types.StringNull(),
		Default:       types.BoolNull(),
	}
}

func TestFilterConfigurationsNoFilters(t *testing.T) {
	state := newState(t)

	configs := []*fastly.CustomTLSConfiguration{
		{ID: "one", Name: "one"},
		{ID: "two", Name: "two"},
	}

	matches, diags := filterConfigurations(context.Background(), configs, &state)
	require.False(t, diags.HasError(), diags)
	require.Len(t, matches, 2)
}

func TestFilterConfigurationsByName(t *testing.T) {
	state := newState(t)
	state.Name = types.StringValue("two")

	configs := []*fastly.CustomTLSConfiguration{
		{ID: "one", Name: "one"},
		{ID: "two", Name: "two"},
	}

	matches, diags := filterConfigurations(context.Background(), configs, &state)
	require.False(t, diags.HasError(), diags)
	require.Len(t, matches, 1)
	require.Equal(t, "two", matches[0].ID)
}

func TestFilterConfigurationsByTLSService(t *testing.T) {
	state := newState(t)
	state.TLSService = types.StringValue(tlsServicePlatform)

	configs := []*fastly.CustomTLSConfiguration{
		{ID: "bulk", Bulk: true},
		{ID: "custom", Bulk: false},
	}

	matches, diags := filterConfigurations(context.Background(), configs, &state)
	require.False(t, diags.HasError(), diags)
	require.Len(t, matches, 1)
	require.Equal(t, "bulk", matches[0].ID)
}

func TestFilterConfigurationsByDefault(t *testing.T) {
	state := newState(t)
	state.Default = types.BoolValue(true)

	configs := []*fastly.CustomTLSConfiguration{
		{ID: "default", Default: true},
		{ID: "not-default", Default: false},
	}

	matches, diags := filterConfigurations(context.Background(), configs, &state)
	require.False(t, diags.HasError(), diags)
	require.Len(t, matches, 1)
	require.Equal(t, "default", matches[0].ID)
}

func TestFilterConfigurationsByProtocols(t *testing.T) {
	state := newState(t)
	tlsProtocols, diags := types.SetValueFrom(context.Background(), types.StringType, []string{"TLSv1.3"})
	require.False(t, diags.HasError(), diags)
	state.TLSProtocols = tlsProtocols

	configs := []*fastly.CustomTLSConfiguration{
		{ID: "matches", TLSProtocols: []string{"TLSv1.2", "TLSv1.3"}},
		{ID: "no-match", TLSProtocols: []string{"TLSv1.2"}},
	}

	matches, diags := filterConfigurations(context.Background(), configs, &state)
	require.False(t, diags.HasError(), diags)
	require.Len(t, matches, 1)
	require.Equal(t, "matches", matches[0].ID)
}

func TestFlattenConfiguration(t *testing.T) {
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	configuration := &fastly.CustomTLSConfiguration{
		ID:            "config-id",
		Name:          "my-config",
		Bulk:          true,
		Default:       true,
		TLSProtocols:  []string{"TLSv1.2", "TLSv1.3"},
		HTTPProtocols: []string{"http/1.1", "h2"},
		CreatedAt:     &createdAt,
		UpdatedAt:     &updatedAt,
		DNSRecords: []*fastly.DNSRecord{
			{ID: "203.0.113.1", RecordType: "A", Region: "global"},
			nil,
		},
	}

	state := newState(t)
	diags := flattenConfiguration(context.Background(), configuration, &state)
	require.False(t, diags.HasError(), diags)

	require.Equal(t, "config-id", state.ID.ValueString())
	require.Equal(t, "my-config", state.Name.ValueString())
	require.Equal(t, tlsServicePlatform, state.TLSService.ValueString())
	require.True(t, state.Default.ValueBool())
	require.Equal(t, createdAt.Format(time.RFC3339), state.CreatedAt.ValueString())
	require.Equal(t, updatedAt.Format(time.RFC3339), state.UpdatedAt.ValueString())
	require.Len(t, state.DNSRecords.Elements(), 1)

	element := state.DNSRecords.Elements()[0]
	object, ok := element.(types.Object)
	require.True(t, ok)
	require.Equal(t, "203.0.113.1", object.Attributes()["record_value"].(types.String).ValueString())
	require.Equal(t, "A", object.Attributes()["record_type"].(types.String).ValueString())
	require.Equal(t, "global", object.Attributes()["region"].(types.String).ValueString())
}
