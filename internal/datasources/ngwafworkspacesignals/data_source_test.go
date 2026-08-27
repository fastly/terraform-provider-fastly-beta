package ngwafworkspacesignals

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_ngwaf_workspace_signals", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.Len(t, resp.Schema.Attributes, 3)

	id, ok := resp.Schema.Attributes["id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, id.Computed)

	workspaceID, ok := resp.Schema.Attributes["workspace_id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, workspaceID.Required)

	signalsAttr, ok := resp.Schema.Attributes["signals"].(datasourceschema.ListNestedAttribute)
	require.True(t, ok)
	require.True(t, signalsAttr.Computed)
	require.Len(t, signalsAttr.NestedObject.Attributes, 4)

	referenceID, ok := signalsAttr.NestedObject.Attributes["reference_id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, referenceID.Computed)
}

func TestFlattenSignals(t *testing.T) {
	remote := []signals.Signal{
		{SignalID: "signal-b", Name: "Signal B", ReferenceID: "site.signal-b", Description: "beta"},
		{SignalID: "signal-a", Name: "Signal A", ReferenceID: "site.signal-a", Description: "alpha"},
	}

	listValue, ids, diags := flattenSignals(remote)
	require.False(t, diags.HasError(), diags)
	require.ElementsMatch(t, []string{"signal-a", "signal-b"}, ids)
	require.Len(t, listValue.Elements(), 2)

	first, ok := listValue.Elements()[0].(types.Object)
	require.True(t, ok)
	firstID, ok := first.Attributes()["id"].(types.String)
	require.True(t, ok)
	require.Equal(t, "signal-a", firstID.ValueString())

	got := make(map[string]map[string]string, len(listValue.Elements()))
	for _, element := range listValue.Elements() {
		object, ok := element.(types.Object)
		require.True(t, ok)

		attributes := object.Attributes()

		id, ok := attributes["id"].(types.String)
		require.True(t, ok)
		name, ok := attributes["name"].(types.String)
		require.True(t, ok)
		referenceID, ok := attributes["reference_id"].(types.String)
		require.True(t, ok)
		description, ok := attributes["description"].(types.String)
		require.True(t, ok)

		got[id.ValueString()] = map[string]string{
			"name":         name.ValueString(),
			"reference_id": referenceID.ValueString(),
			"description":  description.ValueString(),
		}
	}

	require.Equal(t, map[string]map[string]string{
		"signal-a": {
			"name":         "Signal A",
			"reference_id": "site.signal-a",
			"description":  "alpha",
		},
		"signal-b": {
			"name":         "Signal B",
			"reference_id": "site.signal-b",
			"description":  "beta",
		},
	}, got)
}

func TestFlattenSignalsEmpty(t *testing.T) {
	listValue, ids, diags := flattenSignals(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, listValue.Elements())
}
