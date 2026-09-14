package packagehash

import (
	"context"
	"encoding/base64"
	"os"
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

	assert.Equal(t, "fastly_package_hash", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	assert.Len(t, resp.Schema.Attributes, 4)

	hash, ok := resp.Schema.Attributes["hash"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	assert.True(t, hash.Computed)
}

func TestWriteTempPackage(t *testing.T) {
	content := base64.StdEncoding.EncodeToString([]byte("package bytes"))

	path, err := writeTempPackage(content)
	require.NoError(t, err)
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "package bytes", string(got))
}

func TestWriteTempPackage_invalidBase64(t *testing.T) {
	_, err := writeTempPackage("not-base64!!!")
	assert.ErrorContains(t, err, "failed to decode base64 content")
}
