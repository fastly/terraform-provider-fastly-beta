package ngwafworkspacetemplatedsignalrules

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_ngwaf_workspace_templated_signal_rules", resp.TypeName)
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

	rulesAttr, ok := resp.Schema.Attributes["rules"].(datasourceschema.ListNestedAttribute)
	require.True(t, ok)
	require.True(t, rulesAttr.Computed)
	require.Len(t, rulesAttr.NestedObject.Attributes, 5)
}

func TestFlattenRules(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC)

	remote := []rules.Rule{
		{
			RuleID:    "rule-b",
			Enabled:   true,
			Actions:   []rules.Action{{Type: "templated_signal", Signal: "LOGINATTEMPT"}},
			CreatedAt: created,
			UpdatedAt: updated,
		},
		{
			RuleID:    "rule-a",
			Enabled:   false,
			Actions:   []rules.Action{{Type: "templated_signal", Signal: "INVITE-FAILURE"}},
			CreatedAt: created,
			UpdatedAt: updated,
		},
	}

	listValue, ids, diags := flattenRules(remote)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, []string{"rule-a", "rule-b"}, ids)
	require.Len(t, listValue.Elements(), 2)

	got := make(map[string]map[string]string, len(listValue.Elements()))
	for _, element := range listValue.Elements() {
		object, ok := element.(types.Object)
		require.True(t, ok)

		attributes := object.Attributes()

		id, ok := attributes["id"].(types.String)
		require.True(t, ok)
		signal, ok := attributes["signal"].(types.String)
		require.True(t, ok)
		enabled, ok := attributes["enabled"].(types.Bool)
		require.True(t, ok)
		createdAt, ok := attributes["created_at"].(types.String)
		require.True(t, ok)
		updatedAt, ok := attributes["updated_at"].(types.String)
		require.True(t, ok)

		got[id.ValueString()] = map[string]string{
			"signal":     signal.ValueString(),
			"enabled":    strconv.FormatBool(enabled.ValueBool()),
			"created_at": createdAt.ValueString(),
			"updated_at": updatedAt.ValueString(),
		}
	}

	require.Equal(t, map[string]map[string]string{
		"rule-a": {
			"signal":     "INVITE-FAILURE",
			"enabled":    "false",
			"created_at": "2026-01-02T03:04:05Z",
			"updated_at": "2026-01-03T04:05:06Z",
		},
		"rule-b": {
			"signal":     "LOGINATTEMPT",
			"enabled":    "true",
			"created_at": "2026-01-02T03:04:05Z",
			"updated_at": "2026-01-03T04:05:06Z",
		},
	}, got)
}

func TestFlattenRulesNoActions(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	listValue, _, diags := flattenRules([]rules.Rule{
		{RuleID: "rule-a", Enabled: true, CreatedAt: created, UpdatedAt: created},
	})
	require.False(t, diags.HasError(), diags)
	require.Len(t, listValue.Elements(), 1)

	object, ok := listValue.Elements()[0].(types.Object)
	require.True(t, ok)
	signal, ok := object.Attributes()["signal"].(types.String)
	require.True(t, ok)
	require.Equal(t, "", signal.ValueString())
}

func TestFlattenRulesEmpty(t *testing.T) {
	listValue, ids, diags := flattenRules(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, listValue.Elements())
}
