package ngwafworkspaceredactions

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/redactions"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_ngwaf_workspace_redactions", resp.TypeName)
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

	redactionsAttr, ok := resp.Schema.Attributes["redactions"].(datasourceschema.ListNestedAttribute)
	require.True(t, ok)
	require.True(t, redactionsAttr.Computed)
	require.Len(t, redactionsAttr.NestedObject.Attributes, 3)

	fieldType, ok := redactionsAttr.NestedObject.Attributes["type"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, fieldType.Computed)
}

func TestFlattenRedactions(t *testing.T) {
	remote := []redactions.Redaction{
		{RedactionID: "redaction-b", Field: "authorization", Type: "request_header"},
		{RedactionID: "redaction-a", Field: "credit-card", Type: "request_parameter"},
	}

	listValue, ids, diags := flattenRedactions(remote)
	require.False(t, diags.HasError(), diags)
	require.ElementsMatch(t, []string{"redaction-a", "redaction-b"}, ids)
	require.Len(t, listValue.Elements(), 2)

	first, ok := listValue.Elements()[0].(types.Object)
	require.True(t, ok)
	firstID, ok := first.Attributes()["id"].(types.String)
	require.True(t, ok)
	require.Equal(t, "redaction-a", firstID.ValueString())

	got := make(map[string]map[string]string, len(listValue.Elements()))
	for _, element := range listValue.Elements() {
		object, ok := element.(types.Object)
		require.True(t, ok)

		attributes := object.Attributes()

		id, ok := attributes["id"].(types.String)
		require.True(t, ok)
		field, ok := attributes["field"].(types.String)
		require.True(t, ok)
		redactionType, ok := attributes["type"].(types.String)
		require.True(t, ok)

		got[id.ValueString()] = map[string]string{
			"field": field.ValueString(),
			"type":  redactionType.ValueString(),
		}
	}

	require.Equal(t, map[string]map[string]string{
		"redaction-a": {
			"field": "credit-card",
			"type":  "request_parameter",
		},
		"redaction-b": {
			"field": "authorization",
			"type":  "request_header",
		},
	}, got)
}

func TestFlattenRedactionsEmpty(t *testing.T) {
	listValue, ids, diags := flattenRedactions(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, listValue.Elements())
}
