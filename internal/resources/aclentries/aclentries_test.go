package aclentries

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

	"github.com/fastly/go-fastly/v17/fastly/computeacls"
)

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_acl_entries", resp.TypeName)
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

	aclID, ok := resp.Schema.Attributes["acl_id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, aclID.Required)
	require.False(t, aclID.Computed)
	require.Len(t, aclID.PlanModifiers, 1)

	entries, ok := resp.Schema.Attributes["entries"].(resourceschema.MapAttribute)
	require.True(t, ok)
	require.True(t, entries.Required)
	require.Equal(t, types.StringType, entries.ElementType)
	require.Len(t, entries.Validators, 1)

	_, hasManageEntries := resp.Schema.Attributes["manage_entries"]
	require.False(t, hasManageEntries)
}

func TestExpandEntries(t *testing.T) {
	ctx := context.Background()

	t.Run("null map returns nil", func(t *testing.T) {
		var diags diag.Diagnostics
		result := expandEntries(ctx, types.MapNull(types.StringType), &diags)

		require.Nil(t, result)
		require.False(t, diags.HasError())
	})

	t.Run("populated map", func(t *testing.T) {
		value, valueDiags := types.MapValue(types.StringType, map[string]attr.Value{
			"192.0.2.0/24":    types.StringValue("ALLOW"),
			"198.51.100.0/24": types.StringValue("BLOCK"),
		})
		require.False(t, valueDiags.HasError())

		var diags diag.Diagnostics
		result := expandEntries(ctx, value, &diags)

		require.False(t, diags.HasError())
		require.Equal(t, map[string]string{
			"192.0.2.0/24":    "ALLOW",
			"198.51.100.0/24": "BLOCK",
		}, result)
	})
}

func TestFlattenEntries(t *testing.T) {
	var diags diag.Diagnostics
	result := flattenEntries(context.Background(), map[string]string{
		"192.0.2.0/24":    "ALLOW",
		"198.51.100.0/24": "BLOCK",
	}, &diags)

	require.False(t, diags.HasError())

	var got map[string]string
	require.False(t, result.ElementsAs(context.Background(), &got, false).HasError())
	require.Equal(t, map[string]string{
		"192.0.2.0/24":    "ALLOW",
		"198.51.100.0/24": "BLOCK",
	}, got)
}

func TestFilterManagedRemoteEntries(t *testing.T) {
	remote := map[string]string{
		"192.0.2.0/24":    "BLOCK",
		"198.51.100.0/24": "ALLOW",
	}
	managed := map[string]string{
		"192.0.2.0/24":   "ALLOW",
		"203.0.113.0/24": "BLOCK",
	}

	require.Equal(t, map[string]string{
		"192.0.2.0/24": "BLOCK",
	}, filterManagedRemoteEntries(remote, managed))
}

func TestFilterManagedRemoteEntriesLargeACL(t *testing.T) {
	const (
		remoteCount  = 1000
		managedCount = 500
	)

	remote := make(map[string]string, remoteCount)
	managed := make(map[string]string, managedCount)
	want := make(map[string]string, managedCount)

	for i := 0; i < remoteCount; i++ {
		prefix := fmt.Sprintf("10.%d.%d.0/24", i/256, i%256)
		action := "ALLOW"
		if i%2 == 0 {
			action = "BLOCK"
		}
		remote[prefix] = action

		if i < managedCount {
			managed[prefix] = "ALLOW"
			want[prefix] = action
		}
	}

	require.Equal(t, want, filterManagedRemoteEntries(remote, managed))
}

func TestBuildBatchEntries(t *testing.T) {
	tests := []struct {
		name           string
		remote         map[string]string
		currentManaged map[string]string
		desired        map[string]string
		want           []*computeacls.BatchComputeACLEntry
	}{
		{
			name: "create new managed prefix without touching external prefixes",
			remote: map[string]string{
				"198.51.100.0/24": "BLOCK",
			},
			desired: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			want: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Action: new("ALLOW"), Operation: new(createOperation)},
			},
		},
		{
			name: "adopt existing prefix and update its action",
			remote: map[string]string{
				"192.0.2.0/24": "BLOCK",
			},
			desired: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			want: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Action: new("ALLOW"), Operation: new(updateOperation)},
			},
		},
		{
			name: "update changed managed prefix",
			remote: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			currentManaged: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			desired: map[string]string{
				"192.0.2.0/24": "BLOCK",
			},
			want: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Action: new("BLOCK"), Operation: new(updateOperation)},
			},
		},
		{
			name: "recreate managed prefix deleted outside Terraform",
			currentManaged: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			desired: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			want: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Action: new("ALLOW"), Operation: new(createOperation)},
			},
		},
		{
			name: "delete only prefix removed from Terraform ownership",
			remote: map[string]string{
				"192.0.2.0/24":    "ALLOW",
				"198.51.100.0/24": "BLOCK",
			},
			currentManaged: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			desired: map[string]string{},
			want: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Operation: new(deleteOperation)},
			},
		},
		{
			name: "no-op when managed prefix already matches",
			remote: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			currentManaged: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			desired: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			want: nil,
		},
		{
			name: "mixed operations are deterministic",
			remote: map[string]string{
				"192.0.2.0/24":    "ALLOW",
				"192.0.3.0/24":    "BLOCK",
				"198.51.100.0/24": "ALLOW",
				"198.51.101.0/24": "BLOCK",
				"203.0.113.0/24":  "ALLOW",
			},
			currentManaged: map[string]string{
				"192.0.2.0/24":    "ALLOW",
				"192.0.3.0/24":    "BLOCK",
				"198.51.100.0/24": "ALLOW",
				"198.51.101.0/24": "BLOCK",
			},
			desired: map[string]string{
				"198.51.100.0/24": "BLOCK",
				"198.51.101.0/24": "ALLOW",
				"203.0.114.0/24":  "BLOCK",
				"203.0.115.0/24":  "ALLOW",
			},
			want: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Operation: new(deleteOperation)},
				{Prefix: new("192.0.3.0/24"), Operation: new(deleteOperation)},
				{Prefix: new("198.51.100.0/24"), Action: new("BLOCK"), Operation: new(updateOperation)},
				{Prefix: new("198.51.101.0/24"), Action: new("ALLOW"), Operation: new(updateOperation)},
				{Prefix: new("203.0.114.0/24"), Action: new("BLOCK"), Operation: new(createOperation)},
				{Prefix: new("203.0.115.0/24"), Action: new("ALLOW"), Operation: new(createOperation)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, buildBatchEntries(tt.remote, tt.currentManaged, tt.desired))
		})
	}
}

func TestEntriesConverged(t *testing.T) {
	tests := []struct {
		name      string
		remote    map[string]string
		batch     []*computeacls.BatchComputeACLEntry
		converged bool
	}{
		{
			name:   "create not yet visible",
			remote: nil,
			batch: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Action: new("ALLOW"), Operation: new(createOperation)},
			},
			converged: false,
		},
		{
			name: "create visible",
			remote: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			batch: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Action: new("ALLOW"), Operation: new(createOperation)},
			},
			converged: true,
		},
		{
			name: "update not yet visible",
			remote: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			batch: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Action: new("BLOCK"), Operation: new(updateOperation)},
			},
			converged: false,
		},
		{
			name: "update visible",
			remote: map[string]string{
				"192.0.2.0/24": "BLOCK",
			},
			batch: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Action: new("BLOCK"), Operation: new(updateOperation)},
			},
			converged: true,
		},
		{
			name: "delete not yet visible",
			remote: map[string]string{
				"192.0.2.0/24": "ALLOW",
			},
			batch: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Operation: new(deleteOperation)},
			},
			converged: false,
		},
		{
			name:   "delete visible",
			remote: nil,
			batch: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Operation: new(deleteOperation)},
			},
			converged: true,
		},
		{
			name: "mixed operations all converged",
			remote: map[string]string{
				"198.51.100.0/24": "ALLOW",
				"203.0.113.0/24":  "BLOCK",
			},
			batch: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Operation: new(deleteOperation)},
				{Prefix: new("198.51.100.0/24"), Action: new("ALLOW"), Operation: new(updateOperation)},
				{Prefix: new("203.0.113.0/24"), Action: new("BLOCK"), Operation: new(createOperation)},
			},
			converged: true,
		},
		{
			name: "mixed operations one still pending",
			remote: map[string]string{
				"192.0.2.0/24":    "ALLOW",
				"198.51.100.0/24": "ALLOW",
				"203.0.113.0/24":  "BLOCK",
			},
			batch: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Operation: new(deleteOperation)},
				{Prefix: new("198.51.100.0/24"), Action: new("ALLOW"), Operation: new(updateOperation)},
				{Prefix: new("203.0.113.0/24"), Action: new("BLOCK"), Operation: new(createOperation)},
			},
			converged: false,
		},
		{
			name: "unrelated remote entries are ignored",
			remote: map[string]string{
				"192.0.2.0/24":    "ALLOW",
				"198.51.100.0/24": "BLOCK",
			},
			batch: []*computeacls.BatchComputeACLEntry{
				{Prefix: new("192.0.2.0/24"), Action: new("ALLOW"), Operation: new(createOperation)},
			},
			converged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.converged, entriesConverged(tt.remote, tt.batch))
		})
	}
}

func TestResourceID(t *testing.T) {
	require.Equal(t, "acl-id/entries", resourceID("acl-id"))
}
