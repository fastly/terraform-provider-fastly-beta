package ngwafthresholds

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	th "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/thresholds"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_ngwaf_thresholds", resp.TypeName)
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

	thresholds, ok := resp.Schema.Attributes["thresholds"].(datasourceschema.SetNestedAttribute)
	require.True(t, ok)
	require.True(t, thresholds.Computed)
	require.Len(t, thresholds.NestedObject.Attributes, 9)
}

func TestFlattenThresholds(t *testing.T) {
	thresholds := &th.Thresholds{
		Data: []th.Threshold{
			{ThresholdID: "threshold-b", Action: "log", Name: "beta", Signal: "SQLI", Interval: 600, Limit: 50, Duration: 43200, DontNotify: true, Enabled: false},
			{ThresholdID: "threshold-a", Action: "block", Name: "alpha", Signal: "XSS", Interval: 3600, Limit: 10, Duration: 86400, DontNotify: false, Enabled: true},
		},
	}

	setValue, ids, diags := flattenThresholds(thresholds)
	require.False(t, diags.HasError(), diags)
	require.ElementsMatch(t, []string{"threshold-a", "threshold-b"}, ids)
	require.Len(t, setValue.Elements(), 2)

	got := make(map[string]string, len(setValue.Elements()))
	for _, element := range setValue.Elements() {
		object, ok := element.(types.Object)
		require.True(t, ok)

		attributes := object.Attributes()
		id, ok := attributes["id"].(types.String)
		require.True(t, ok)
		name, ok := attributes["name"].(types.String)
		require.True(t, ok)

		got[id.ValueString()] = name.ValueString()
	}

	require.Equal(t, map[string]string{
		"threshold-a": "alpha",
		"threshold-b": "beta",
	}, got)
}

func TestFlattenThresholdsEmpty(t *testing.T) {
	setValue, ids, diags := flattenThresholds(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, setValue.Elements())
}
