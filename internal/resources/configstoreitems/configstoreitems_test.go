package configstoreitems

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

	require.Equal(t, "fastly_configstore_items", resp.TypeName)
}

func TestSchema(t *testing.T) {
	r := NewResource()

	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	require.Len(t, resp.Schema.Attributes, 3)

	id, ok := resp.Schema.Attributes["id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, id.Computed)
	require.False(t, id.Required)
	require.Len(t, id.PlanModifiers, 1)

	storeID, ok := resp.Schema.Attributes["store_id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, storeID.Required)
	require.False(t, storeID.Computed)
	require.NotEmpty(t, storeID.Validators)
	require.Len(t, storeID.PlanModifiers, 1)

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

	for i := 0; i < remoteCount; i++ {
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
		want           []*fastly.BatchConfigStoreItem
	}{
		{
			name: "create new managed key without touching external keys",
			remote: map[string]string{
				"external": "external-value",
			},
			desired: map[string]string{
				"managed": "managed-value",
			},
			want: []*fastly.BatchConfigStoreItem{
				{Operation: fastly.CreateBatchOperation, ItemKey: "managed", ItemValue: "managed-value"},
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
			want: []*fastly.BatchConfigStoreItem{
				{Operation: fastly.UpdateBatchOperation, ItemKey: "managed", ItemValue: "terraform-value"},
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
			want: []*fastly.BatchConfigStoreItem{
				{Operation: fastly.UpdateBatchOperation, ItemKey: "managed", ItemValue: "new"},
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
			want: []*fastly.BatchConfigStoreItem{
				{Operation: fastly.CreateBatchOperation, ItemKey: "managed", ItemValue: "value"},
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
			want: []*fastly.BatchConfigStoreItem{
				{Operation: fastly.DeleteBatchOperation, ItemKey: "managed"},
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
			want: []*fastly.BatchConfigStoreItem{
				{Operation: fastly.DeleteBatchOperation, ItemKey: "delete-a"},
				{Operation: fastly.DeleteBatchOperation, ItemKey: "delete-b"},
				{Operation: fastly.CreateBatchOperation, ItemKey: "create-a", ItemValue: "new"},
				{Operation: fastly.CreateBatchOperation, ItemKey: "create-b", ItemValue: "new"},
				{Operation: fastly.UpdateBatchOperation, ItemKey: "update-a", ItemValue: "new"},
				{Operation: fastly.UpdateBatchOperation, ItemKey: "update-b", ItemValue: "new"},
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
	require.Equal(t, "store-id/items", resourceID("store-id"))
}
