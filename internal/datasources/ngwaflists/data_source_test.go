package ngwaflists

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)

	require.Equal(t, "fastly_ngwaf_lists", resp.TypeName)
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

	listsAttr, ok := resp.Schema.Attributes["lists"].(datasourceschema.ListNestedAttribute)
	require.True(t, ok)
	require.True(t, listsAttr.Computed)
	require.Len(t, listsAttr.NestedObject.Attributes, 7)
}

func TestAccountListsListInput(t *testing.T) {
	input := accountListsListInput()

	require.NotNil(t, input.Scope)
	require.Equal(t, scope.ScopeTypeAccount, input.Scope.Type)
	require.Empty(t, input.Scope.AppliesTo)
}

func TestFlattenListsSortsByIDWithoutMutatingInput(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC)

	remote := []lists.List{
		{
			ListID:      "list-b",
			Type:        "string",
			ReferenceID: "account.list-b",
			Name:        "List B",
			Description: "beta",
			CreatedAt:   created,
			UpdatedAt:   updated,
		},
		{
			ListID:      "list-a",
			Type:        "ip",
			ReferenceID: "account.list-a",
			Name:        "List A",
			Description: "alpha",
			CreatedAt:   created,
			UpdatedAt:   updated,
		},
	}

	listValue, ids, diags := FlattenLists(remote)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, []string{"list-a", "list-b"}, ids)
	require.Len(t, listValue.Elements(), 2)

	// FlattenLists sorts a copy, not the API response slice.
	require.Equal(t, "list-b", remote[0].ListID)
	require.Equal(t, "list-a", remote[1].ListID)

	first, ok := listValue.Elements()[0].(types.Object)
	require.True(t, ok)
	firstID, ok := first.Attributes()["id"].(types.String)
	require.True(t, ok)
	require.Equal(t, "list-a", firstID.ValueString())

	got := make(map[string]map[string]string, len(listValue.Elements()))
	for _, element := range listValue.Elements() {
		object, ok := element.(types.Object)
		require.True(t, ok)

		attributes := object.Attributes()
		id := attributes["id"].(types.String)
		name := attributes["name"].(types.String)
		listType := attributes["type"].(types.String)
		referenceID := attributes["reference_id"].(types.String)
		description := attributes["description"].(types.String)
		createdAt := attributes["created_at"].(types.String)
		updatedAt := attributes["updated_at"].(types.String)

		got[id.ValueString()] = map[string]string{
			"name":         name.ValueString(),
			"type":         listType.ValueString(),
			"reference_id": referenceID.ValueString(),
			"description":  description.ValueString(),
			"created_at":   createdAt.ValueString(),
			"updated_at":   updatedAt.ValueString(),
		}
	}

	require.Equal(t, map[string]map[string]string{
		"list-a": {
			"name":         "List A",
			"type":         "ip",
			"reference_id": "account.list-a",
			"description":  "alpha",
			"created_at":   "2026-01-02T03:04:05Z",
			"updated_at":   "2026-01-03T04:05:06Z",
		},
		"list-b": {
			"name":         "List B",
			"type":         "string",
			"reference_id": "account.list-b",
			"description":  "beta",
			"created_at":   "2026-01-02T03:04:05Z",
			"updated_at":   "2026-01-03T04:05:06Z",
		},
	}, got)
}

func TestFlattenListsEmpty(t *testing.T) {
	listValue, ids, diags := FlattenLists(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, listValue.Elements())
}
