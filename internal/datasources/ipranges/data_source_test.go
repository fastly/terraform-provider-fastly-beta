package ipranges

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRead_notConfigured(t *testing.T) {
	d := &DataSource{}

	var resp datasource.ReadResponse
	d.Read(context.Background(), datasource.ReadRequest{}, &resp)

	require.True(t, resp.Diagnostics.HasError())
}

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	assert.Equal(t, "fastly_ip_ranges", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assert.Len(t, resp.Schema.Attributes, 3)

	cidrBlocks, ok := resp.Schema.Attributes["cidr_blocks"].(datasourceschema.ListAttribute)
	require.True(t, ok)
	assert.True(t, cidrBlocks.Computed)

	ipv6CIDRBlocks, ok := resp.Schema.Attributes["ipv6_cidr_blocks"].(datasourceschema.ListAttribute)
	require.True(t, ok)
	assert.True(t, ipv6CIDRBlocks.Computed)
}

func TestFlattenIPRanges_sorts(t *testing.T) {
	var state DataSourceModel
	diags := flattenIPRanges(context.Background(), []string{"3.0.0.0/8", "1.0.0.0/8", "2.0.0.0/8"}, []string{"::2/128", "::1/128"}, &state)
	require.False(t, diags.HasError(), diags)

	var cidrBlocks, ipv6CIDRBlocks []string
	require.False(t, state.CIDRBlocks.ElementsAs(context.Background(), &cidrBlocks, false).HasError())
	require.False(t, state.IPv6CIDRBlocks.ElementsAs(context.Background(), &ipv6CIDRBlocks, false).HasError())

	assert.Equal(t, []string{"1.0.0.0/8", "2.0.0.0/8", "3.0.0.0/8"}, cidrBlocks)
	assert.Equal(t, []string{"::1/128", "::2/128"}, ipv6CIDRBlocks)
}

func TestFlattenIPRanges_idStableRegardlessOfInputOrder(t *testing.T) {
	var stateA, stateB DataSourceModel

	diags := flattenIPRanges(context.Background(), []string{"1.0.0.0/8", "2.0.0.0/8"}, []string{"::1/128"}, &stateA)
	require.False(t, diags.HasError(), diags)

	diags = flattenIPRanges(context.Background(), []string{"2.0.0.0/8", "1.0.0.0/8"}, []string{"::1/128"}, &stateB)
	require.False(t, diags.HasError(), diags)

	assert.Equal(t, stateA.ID.ValueString(), stateB.ID.ValueString())
	assert.NotEmpty(t, stateA.ID.ValueString())
}

func TestFlattenIPRanges_idChangesWithContent(t *testing.T) {
	var stateA, stateB DataSourceModel

	diags := flattenIPRanges(context.Background(), []string{"1.0.0.0/8"}, nil, &stateA)
	require.False(t, diags.HasError(), diags)

	diags = flattenIPRanges(context.Background(), []string{"1.0.0.0/8", "2.0.0.0/8"}, nil, &stateB)
	require.False(t, diags.HasError(), diags)

	assert.NotEqual(t, stateA.ID.ValueString(), stateB.ID.ValueString())
}
