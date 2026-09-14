package ipranges

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
