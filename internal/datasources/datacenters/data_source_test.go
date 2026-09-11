package datacenters

import (
	"context"
	"testing"

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

	require.Equal(t, "fastly_datacenters", resp.TypeName)
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

	pops, ok := resp.Schema.Attributes["pops"].(datasourceschema.SetNestedAttribute)
	require.True(t, ok)
	require.True(t, pops.Computed)
	require.Len(t, pops.NestedObject.Attributes, 5)

	for _, name := range []string{"code", "group", "name", "shield"} {
		attr, ok := pops.NestedObject.Attributes[name].(datasourceschema.StringAttribute)
		require.True(t, ok, name)
		require.True(t, attr.Computed, name)
	}

	coordinates, ok := pops.NestedObject.Attributes["coordinates"].(datasourceschema.SingleNestedAttribute)
	require.True(t, ok)
	require.True(t, coordinates.Computed)
	require.Len(t, coordinates.Attributes, 4)

	for _, name := range []string{"latitude", "longitude", "x", "y"} {
		attr, ok := coordinates.Attributes[name].(datasourceschema.Float64Attribute)
		require.True(t, ok, name)
		require.True(t, attr.Computed, name)
	}
}

func TestFlattenDatacenters(t *testing.T) {
	datacenters := []fastly.Datacenter{
		{
			Code: fastly.ToPointer("SEA"), Group: fastly.ToPointer("US"), Name: fastly.ToPointer("Seattle"), Shield: fastly.ToPointer("seattle-va-us"),
			Coordinates: &fastly.Coordinates{
				Latitude:  fastly.ToPointer(47.6062),
				Longitude: fastly.ToPointer(-122.3321),
				X:         fastly.ToPointer(100.0),
				Y:         fastly.ToPointer(200.0),
			},
		},
		{Code: fastly.ToPointer("LHR"), Group: fastly.ToPointer("EU"), Name: fastly.ToPointer("London")},
	}

	setValue, ids, diags := flattenDatacenters(datacenters)
	require.False(t, diags.HasError(), diags)
	require.ElementsMatch(t, []string{"SEA", "LHR"}, ids)
	require.Len(t, setValue.Elements(), 2)

	got := make(map[string]string, len(setValue.Elements()))
	for _, element := range setValue.Elements() {
		object, ok := element.(types.Object)
		require.True(t, ok)

		attributes := object.Attributes()
		code, ok := attributes["code"].(types.String)
		require.True(t, ok)
		name, ok := attributes["name"].(types.String)
		require.True(t, ok)

		got[code.ValueString()] = name.ValueString()

		coordinates, ok := attributes["coordinates"].(types.Object)
		require.True(t, ok)

		switch code.ValueString() {
		case "LHR":
			shield, ok := attributes["shield"].(types.String)
			require.True(t, ok)
			require.True(t, shield.IsNull())
			require.True(t, coordinates.IsNull())
		case "SEA":
			latitude, ok := coordinates.Attributes()["latitude"].(types.Float64)
			require.True(t, ok)
			require.Equal(t, 47.6062, latitude.ValueFloat64())
		}
	}

	require.Equal(t, map[string]string{
		"SEA": "Seattle",
		"LHR": "London",
	}, got)
}

func TestFlattenDatacentersEmpty(t *testing.T) {
	setValue, ids, diags := flattenDatacenters(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, setValue.Elements())
}
