package tlsplatformcertificateids

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/stretchr/testify/assert"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)

	assert.Equal(t, "fastly_tls_platform_certificate_ids", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	assert.False(t, resp.Diagnostics.HasError())
	assert.Len(t, resp.Schema.Attributes, 2)
}
