package ngwafworkspacevirtualpatches

import (
	"context"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	vp "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/virtualpatches"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_ngwaf_workspace_virtual_patches", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.Len(t, resp.Schema.Attributes, 3)

	workspaceID, ok := resp.Schema.Attributes["workspace_id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, workspaceID.Required)

	virtualPatches, ok := resp.Schema.Attributes["virtual_patches"].(datasourceschema.ListNestedAttribute)
	require.True(t, ok)
	require.True(t, virtualPatches.Computed)
	require.Len(t, virtualPatches.NestedObject.Attributes, 4)
}

func TestFlattenVirtualPatches(t *testing.T) {
	virtualPatches := &vp.VirtualPatches{
		Data: []vp.VirtualPatch{
			{ID: "CVE-2021-44228", Description: "Log4Shell", Mode: "log", Enabled: false},
			{ID: "CVE-2017-5638", Description: "Apache Struts", Mode: "block", Enabled: true},
		},
	}

	listValue, ids, diags := flattenVirtualPatches(virtualPatches)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, []string{"CVE-2017-5638", "CVE-2021-44228"}, ids)
	require.Len(t, listValue.Elements(), 2)

	first, ok := listValue.Elements()[0].(types.Object)
	require.True(t, ok)
	firstID, ok := first.Attributes()["id"].(types.String)
	require.True(t, ok)
	require.Equal(t, "CVE-2017-5638", firstID.ValueString())

	got := make(map[string]map[string]string, len(listValue.Elements()))
	for _, element := range listValue.Elements() {
		object, ok := element.(types.Object)
		require.True(t, ok)

		attributes := object.Attributes()
		id, ok := attributes["id"].(types.String)
		require.True(t, ok)
		mode, ok := attributes["mode"].(types.String)
		require.True(t, ok)
		description, ok := attributes["description"].(types.String)
		require.True(t, ok)
		enabled, ok := attributes["enabled"].(types.Bool)
		require.True(t, ok)

		got[id.ValueString()] = map[string]string{
			"mode":        mode.ValueString(),
			"description": description.ValueString(),
			"enabled":     strconv.FormatBool(enabled.ValueBool()),
		}
	}

	require.Equal(t, map[string]map[string]string{
		"CVE-2017-5638": {
			"mode":        "block",
			"description": "Apache Struts",
			"enabled":     "true",
		},
		"CVE-2021-44228": {
			"mode":        "log",
			"description": "Log4Shell",
			"enabled":     "false",
		},
	}, got)
}

func TestFlattenVirtualPatchesEmpty(t *testing.T) {
	listValue, ids, diags := flattenVirtualPatches(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, listValue.Elements())
}
