package servicedictionaryitems

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_service_dictionary_items", resp.TypeName)
}

func TestSchema(t *testing.T) {
	r := NewResource()

	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	require.Len(t, resp.Schema.Attributes, 4)

	id, ok := resp.Schema.Attributes["id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, id.Computed)
	require.False(t, id.Required)
	require.Len(t, id.PlanModifiers, 1)

	serviceID, ok := resp.Schema.Attributes["service_id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, serviceID.Required)
	require.False(t, serviceID.Computed)
	require.NotEmpty(t, serviceID.Validators)
	require.Len(t, serviceID.PlanModifiers, 1)

	dictionaryID, ok := resp.Schema.Attributes["dictionary_id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, dictionaryID.Required)
	require.False(t, dictionaryID.Computed)
	require.NotEmpty(t, dictionaryID.Validators)
	require.Len(t, dictionaryID.PlanModifiers, 1)

	items, ok := resp.Schema.Attributes["items"].(resourceschema.MapAttribute)
	require.True(t, ok)
	require.True(t, items.Required)
	require.Equal(t, types.StringType, items.ElementType)
	require.Len(t, items.Validators, 1)
}

func TestExpandItems(t *testing.T) {
	ctx := context.Background()

	t.Run("null map returns nil", func(t *testing.T) {
		var diags diag.Diagnostics
		got := expandItems(ctx, types.MapNull(types.StringType), &diags)

		require.Nil(t, got)
		require.False(t, diags.HasError())
	})

	t.Run("populated map", func(t *testing.T) {
		value, valueDiags := types.MapValue(types.StringType, map[string]attr.Value{
			"key-1": types.StringValue("value-1"),
			"key-2": types.StringValue("value-2"),
		})
		require.False(t, valueDiags.HasError())

		var diags diag.Diagnostics
		got := expandItems(ctx, value, &diags)

		require.False(t, diags.HasError())
		require.Equal(t, map[string]string{
			"key-1": "value-1",
			"key-2": "value-2",
		}, got)
	})
}

func TestFlattenItems(t *testing.T) {
	var diags diag.Diagnostics
	got := flattenItems(context.Background(), map[string]string{
		"key-1": "value-1",
		"key-2": "value-2",
	}, &diags)

	require.False(t, diags.HasError())

	var result map[string]string
	require.False(t, got.ElementsAs(context.Background(), &result, false).HasError())
	require.Equal(t, map[string]string{
		"key-1": "value-1",
		"key-2": "value-2",
	}, result)
}

func TestFilterManagedRemoteItems(t *testing.T) {
	remote := map[string]string{
		"managed":  "remote-value",
		"external": "leave-me-alone",
	}
	managed := map[string]string{
		"managed": "old-state-value",
		"missing": "was-deleted-externally",
	}

	require.Equal(t, map[string]string{
		"managed": "remote-value",
	}, filterManagedRemoteItems(remote, managed))
}

func TestFilterManagedRemoteItemsLargeStore(t *testing.T) {
	const (
		remoteCount  = 500
		managedCount = 250
	)

	remote := make(map[string]string, remoteCount)
	managed := make(map[string]string, managedCount)
	want := make(map[string]string, managedCount)

	for i := range remoteCount {
		key := fmt.Sprintf("key-%03d", i)
		value := fmt.Sprintf("remote-%03d", i)
		remote[key] = value

		if i < managedCount {
			managed[key] = "previous-state-value"
			want[key] = value
		}
	}

	got := filterManagedRemoteItems(remote, managed)

	require.Len(t, got, managedCount)
	require.Equal(t, want, got)
}

func TestBuildBatchOperations(t *testing.T) {
	tests := []struct {
		name           string
		remote         map[string]string
		currentManaged map[string]string
		desired        map[string]string
		want           []*fastly.BatchDictionaryItem
	}{
		{
			name: "create new managed key without touching external keys",
			remote: map[string]string{
				"external": "external-value",
			},
			desired: map[string]string{
				"managed": "managed-value",
			},
			want: []*fastly.BatchDictionaryItem{
				{Operation: new(fastly.CreateBatchOperation), ItemKey: new("managed"), ItemValue: new("managed-value")},
			},
		},
		{
			name: "adopt existing key and update its value",
			remote: map[string]string{
				"managed": "external-value",
			},
			desired: map[string]string{
				"managed": "terraform-value",
			},
			want: []*fastly.BatchDictionaryItem{
				{Operation: new(fastly.UpdateBatchOperation), ItemKey: new("managed"), ItemValue: new("terraform-value")},
			},
		},
		{
			name: "update changed managed key",
			remote: map[string]string{
				"managed": "old",
			},
			currentManaged: map[string]string{
				"managed": "old",
			},
			desired: map[string]string{
				"managed": "new",
			},
			want: []*fastly.BatchDictionaryItem{
				{Operation: new(fastly.UpdateBatchOperation), ItemKey: new("managed"), ItemValue: new("new")},
			},
		},
		{
			name: "recreate managed key deleted outside Terraform",
			currentManaged: map[string]string{
				"managed": "value",
			},
			desired: map[string]string{
				"managed": "value",
			},
			want: []*fastly.BatchDictionaryItem{
				{Operation: new(fastly.CreateBatchOperation), ItemKey: new("managed"), ItemValue: new("value")},
			},
		},
		{
			name: "delete only key removed from Terraform ownership",
			remote: map[string]string{
				"managed":  "value",
				"external": "leave-me-alone",
			},
			currentManaged: map[string]string{
				"managed": "value",
			},
			desired: map[string]string{},
			want: []*fastly.BatchDictionaryItem{
				{Operation: new(fastly.DeleteBatchOperation), ItemKey: new("managed")},
			},
		},
		{
			name: "no-op when managed key already matches",
			remote: map[string]string{
				"managed": "value",
			},
			currentManaged: map[string]string{
				"managed": "value",
			},
			desired: map[string]string{
				"managed": "value",
			},
			want: nil,
		},
		{
			name: "mixed operations are deterministic",
			remote: map[string]string{
				"delete-b": "value",
				"delete-a": "value",
				"update-b": "old",
				"update-a": "old",
				"external": "leave-me-alone",
			},
			currentManaged: map[string]string{
				"delete-b": "value",
				"delete-a": "value",
				"update-b": "old",
				"update-a": "old",
			},
			desired: map[string]string{
				"update-b": "new",
				"update-a": "new",
				"create-b": "new",
				"create-a": "new",
			},
			want: []*fastly.BatchDictionaryItem{
				{Operation: new(fastly.DeleteBatchOperation), ItemKey: new("delete-a")},
				{Operation: new(fastly.DeleteBatchOperation), ItemKey: new("delete-b")},
				{Operation: new(fastly.CreateBatchOperation), ItemKey: new("create-a"), ItemValue: new("new")},
				{Operation: new(fastly.CreateBatchOperation), ItemKey: new("create-b"), ItemValue: new("new")},
				{Operation: new(fastly.UpdateBatchOperation), ItemKey: new("update-a"), ItemValue: new("new")},
				{Operation: new(fastly.UpdateBatchOperation), ItemKey: new("update-b"), ItemValue: new("new")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, buildBatchOperations(tt.remote, tt.currentManaged, tt.desired))
		})
	}
}

func TestResourceID(t *testing.T) {
	require.Equal(t, "service-id/dictionary-id", resourceID("service-id", "dictionary-id"))
}
