package ngwafsignals

import (
	"context"
	"strconv"
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

	require.Equal(t, "fastly_ngwaf_signals", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	require.Len(t, resp.Schema.Attributes, 2)

	id, ok := resp.Schema.Attributes["id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, id.Computed)

	signalsAttr, ok := resp.Schema.Attributes["signals"].(datasourceschema.ListNestedAttribute)
	require.True(t, ok)
	require.True(t, signalsAttr.Computed)
	require.Len(t, signalsAttr.NestedObject.Attributes, 5)

	referenceID, ok := signalsAttr.NestedObject.Attributes["reference_id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, referenceID.Computed)

	appliesTo, ok := signalsAttr.NestedObject.Attributes["applies_to"].(datasourceschema.SetAttribute)
	require.True(t, ok)
	require.True(t, appliesTo.Computed)
}

func TestAccountSignalsListInput(t *testing.T) {
	input := accountSignalsListInput()

	require.NotNil(t, input.Limit)
	require.Equal(t, listLimit, *input.Limit)
	require.NotNil(t, input.Scope)
	require.Equal(t, "account", string(input.Scope.Type))
	require.Empty(t, input.Scope.AppliesTo)
}

func TestFlattenSignalsSortsByIDWithoutMutatingInput(t *testing.T) {
	remote := []signals.Signal{
		{
			SignalID:    "signal-b",
			Name:        "Signal B",
			ReferenceID: "site.signal-b",
			Description: "beta",
			Scope: signals.Scope{
				Type:      "account",
				AppliesTo: []string{"*"},
			},
		},
		{
			SignalID:    "signal-a",
			Name:        "Signal A",
			ReferenceID: "site.signal-a",
			Description: "alpha",
			Scope: signals.Scope{
				Type:      "account",
				AppliesTo: []string{"workspace-one", "workspace-two"},
			},
		},
	}

	listValue, ids, diags := flattenSignals(context.Background(), remote)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, []string{"signal-a", "signal-b"}, ids)
	require.Len(t, listValue.Elements(), 2)

	// flattenSignals sorts a copy, not the API response slice.
	require.Equal(t, "signal-b", remote[0].SignalID)
	require.Equal(t, "signal-a", remote[1].SignalID)

	first, ok := listValue.Elements()[0].(types.Object)
	require.True(t, ok)
	firstID, ok := first.Attributes()["id"].(types.String)
	require.True(t, ok)
	require.Equal(t, "signal-a", firstID.ValueString())

	got := map[string]map[string]string{}
	for _, element := range listValue.Elements() {
		object, ok := element.(types.Object)
		require.True(t, ok)

		attributes := object.Attributes()
		id, ok := attributes["id"].(types.String)
		require.True(t, ok)
		name, ok := attributes["name"].(types.String)
		require.True(t, ok)
		description, ok := attributes["description"].(types.String)
		require.True(t, ok)
		referenceID, ok := attributes["reference_id"].(types.String)
		require.True(t, ok)
		appliesTo, ok := attributes["applies_to"].(types.Set)
		require.True(t, ok)

		got[id.ValueString()] = map[string]string{
			"name":         name.ValueString(),
			"description":  description.ValueString(),
			"reference_id": referenceID.ValueString(),
			"applies_to.#": strconv.Itoa(len(appliesTo.Elements())),
		}
	}

	require.Equal(t, map[string]map[string]string{
		"signal-a": {
			"name":         "Signal A",
			"description":  "alpha",
			"reference_id": "site.signal-a",
			"applies_to.#": "2",
		},
		"signal-b": {
			"name":         "Signal B",
			"description":  "beta",
			"reference_id": "site.signal-b",
			"applies_to.#": "1",
		},
	}, got)
}

func TestFlattenSignalsEmpty(t *testing.T) {
	listValue, ids, diags := flattenSignals(context.Background(), nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, listValue.Elements())
}
