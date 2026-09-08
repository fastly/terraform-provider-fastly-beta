package ngwafaccountrules

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_ngwaf_rules", resp.TypeName)
}

func TestSchema(t *testing.T) {
	d := NewDataSource()

	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	// Account scope takes no input: the endpoint is not parameterised by a
	// workspace the way fastly_ngwaf_workspace_rules is.
	require.Len(t, resp.Schema.Attributes, 2)

	id, ok := resp.Schema.Attributes["id"].(datasourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, id.Computed)

	rulesAttr, ok := resp.Schema.Attributes["rules"].(datasourceschema.ListNestedAttribute)
	require.True(t, ok)
	require.True(t, rulesAttr.Computed)
	require.Len(t, rulesAttr.NestedObject.Attributes, 7)
}

func TestFlattenRules(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC)

	remote := []rules.Rule{
		{
			RuleID:      "rule-b",
			Type:        "signal",
			Description: "beta",
			Enabled:     true,
			Scope:       rules.Scope{Type: "account", AppliesTo: []string{"*"}},
			CreatedAt:   created,
			UpdatedAt:   updated,
		},
		{
			RuleID:      "rule-a",
			Type:        "request",
			Description: "alpha",
			Enabled:     false,
			Scope:       rules.Scope{Type: "account", AppliesTo: []string{"ws1", "ws2"}},
			CreatedAt:   created,
			UpdatedAt:   updated,
		},
	}

	listVal, ids, diags := flattenRules(context.Background(), remote)
	require.False(t, diags.HasError(), diags)

	// Sorted by rule ID so the data source is stable across API orderings.
	require.Equal(t, []string{"rule-a", "rule-b"}, ids)
	require.Len(t, listVal.Elements(), 2)
}
