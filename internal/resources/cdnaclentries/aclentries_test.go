package cdnaclentries

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestFlattenEntries(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		remote []*fastly.ACLEntry
		want   int
	}{
		{
			name: "multiple entries",
			remote: []*fastly.ACLEntry{
				{
					EntryID: new("entry1"),
					IP:      new("127.0.0.1"),
					Subnet:  new(24),
					Negated: new(false),
					Comment: new("ACL Entry 1"),
				},
				{
					EntryID: new("entry2"),
					IP:      new("192.168.0.1"),
					Subnet:  new(16),
					Negated: new(true),
					Comment: new("ACL Entry 2"),
				},
			},
			want: 2,
		},
		{
			name:   "empty entries",
			remote: []*fastly.ACLEntry{},
			want:   0,
		},
		{
			name:   "nil entries",
			remote: nil,
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			result := flattenEntries(ctx, tt.remote, types.SetNull(types.ObjectType{AttrTypes: entryAttrTypes}), &diags)

			assert.False(t, diags.HasError())

			if tt.want == 0 {
				assert.True(t, result.IsNull() || len(result.Elements()) == 0)
			} else {
				assert.Equal(t, tt.want, len(result.Elements()))
			}

			if tt.name == "multiple entries" && len(result.Elements()) > 0 {
				elements := result.Elements()
				firstEntry := elements[0].(types.Object)
				attrs := firstEntry.Attributes()

				assert.Equal(t, "entry1", attrs["id"].(types.String).ValueString())
				assert.Equal(t, "127.0.0.1", attrs["ip"].(types.String).ValueString())
				assert.Equal(t, int64(24), attrs["subnet"].(types.Int64).ValueInt64())
				assert.Equal(t, false, attrs["negated"].(types.Bool).ValueBool())
				assert.Equal(t, "ACL Entry 1", attrs["comment"].(types.String).ValueString())
			}
		})
	}
}

func TestFlattenEntries_NullPreservation(t *testing.T) {
	ctx := context.Background()

	t.Run("preserves null negated when planned as null", func(t *testing.T) {
		// API returns negated=false even when user didn't specify it
		remote := []*fastly.ACLEntry{
			{
				EntryID: new("entry1"),
				IP:      new("10.0.0.1"),
				Negated: new(false), // API default
			},
		}

		// User's plan had negated=null (omitted)
		plannedEntry := types.ObjectValueMust(
			entryAttrTypes,
			map[string]attr.Value{
				"id":      types.StringNull(),
				"ip":      types.StringValue("10.0.0.1"),
				"subnet":  types.Int64Null(),
				"negated": types.BoolNull(), // User didn't specify
				"comment": types.StringNull(),
			},
		)
		plannedEntries := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{plannedEntry})

		var diags diag.Diagnostics
		result := flattenEntries(ctx, remote, plannedEntries, &diags)

		assert.False(t, diags.HasError())
		elements := result.Elements()
		assert.Equal(t, 1, len(elements))

		entry := elements[0].(types.Object)
		attrs := entry.Attributes()

		// Should preserve null from plan, not use API's false
		assert.True(t, attrs["negated"].(types.Bool).IsNull(), "negated should be null when it was null in plan")
	})

	t.Run("preserves explicit negated value when planned", func(t *testing.T) {
		remote := []*fastly.ACLEntry{
			{
				EntryID: new("entry1"),
				IP:      new("10.0.0.1"),
				Negated: new(true),
			},
		}

		// User explicitly set negated=true
		plannedEntry := types.ObjectValueMust(
			entryAttrTypes,
			map[string]attr.Value{
				"id":      types.StringNull(),
				"ip":      types.StringValue("10.0.0.1"),
				"subnet":  types.Int64Null(),
				"negated": types.BoolValue(true), // User specified
				"comment": types.StringNull(),
			},
		)
		plannedEntries := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{plannedEntry})

		var diags diag.Diagnostics
		result := flattenEntries(ctx, remote, plannedEntries, &diags)

		assert.False(t, diags.HasError())
		elements := result.Elements()
		entry := elements[0].(types.Object)
		attrs := entry.Attributes()

		// Should use the explicit value
		assert.False(t, attrs["negated"].(types.Bool).IsNull())
		assert.True(t, attrs["negated"].(types.Bool).ValueBool())
	})

	t.Run("preserves null subnet when planned as null", func(t *testing.T) {
		remote := []*fastly.ACLEntry{
			{
				EntryID: new("entry1"),
				IP:      new("10.0.0.1"),
				Subnet:  new(32), // API may compute default
			},
		}

		plannedEntry := types.ObjectValueMust(
			entryAttrTypes,
			map[string]attr.Value{
				"id":      types.StringNull(),
				"ip":      types.StringValue("10.0.0.1"),
				"subnet":  types.Int64Null(), // User didn't specify
				"negated": types.BoolNull(),
				"comment": types.StringNull(),
			},
		)
		plannedEntries := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{plannedEntry})

		var diags diag.Diagnostics
		result := flattenEntries(ctx, remote, plannedEntries, &diags)

		assert.False(t, diags.HasError())
		elements := result.Elements()
		entry := elements[0].(types.Object)
		attrs := entry.Attributes()

		assert.True(t, attrs["subnet"].(types.Int64).IsNull(), "subnet should be null when it was null in plan")
	})

	t.Run("preserves null comment when planned as null", func(t *testing.T) {
		remote := []*fastly.ACLEntry{
			{
				EntryID: new("entry1"),
				IP:      new("10.0.0.1"),
				Comment: new(""), // API returns empty string
			},
		}

		plannedEntry := types.ObjectValueMust(
			entryAttrTypes,
			map[string]attr.Value{
				"id":      types.StringNull(),
				"ip":      types.StringValue("10.0.0.1"),
				"subnet":  types.Int64Null(),
				"negated": types.BoolNull(),
				"comment": types.StringNull(), // User didn't specify
			},
		)
		plannedEntries := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{plannedEntry})

		var diags diag.Diagnostics
		result := flattenEntries(ctx, remote, plannedEntries, &diags)

		assert.False(t, diags.HasError())
		elements := result.Elements()
		entry := elements[0].(types.Object)
		attrs := entry.Attributes()

		assert.True(t, attrs["comment"].(types.String).IsNull(), "comment should be null when it was null in plan")
	})

	t.Run("uses API values when no plan provided", func(t *testing.T) {
		// This happens during import or initial read
		remote := []*fastly.ACLEntry{
			{
				EntryID: new("entry1"),
				IP:      new("10.0.0.1"),
				Subnet:  new(24),
				Negated: new(false),
				Comment: new("imported entry"),
			},
		}

		// No plan (null set)
		plannedEntries := types.SetNull(types.ObjectType{AttrTypes: entryAttrTypes})

		var diags diag.Diagnostics
		result := flattenEntries(ctx, remote, plannedEntries, &diags)

		assert.False(t, diags.HasError())
		elements := result.Elements()
		entry := elements[0].(types.Object)
		attrs := entry.Attributes()

		// Should use all API values since there's no plan to preserve nulls from
		assert.Equal(t, int64(24), attrs["subnet"].(types.Int64).ValueInt64())
		assert.Equal(t, false, attrs["negated"].(types.Bool).ValueBool())
		assert.Equal(t, "imported entry", attrs["comment"].(types.String).ValueString())
	})

	t.Run("handles multiple entries with mixed null preservation", func(t *testing.T) {
		remote := []*fastly.ACLEntry{
			{
				EntryID: new("entry1"),
				IP:      new("10.0.0.1"),
				Negated: new(false),
				Comment: new(""),
			},
			{
				EntryID: new("entry2"),
				IP:      new("10.0.0.2"),
				Negated: new(true),
				Comment: new("explicit comment"),
			},
		}

		// First entry has nulls, second has explicit values
		entry1 := types.ObjectValueMust(
			entryAttrTypes,
			map[string]attr.Value{
				"id":      types.StringNull(),
				"ip":      types.StringValue("10.0.0.1"),
				"subnet":  types.Int64Null(),
				"negated": types.BoolNull(),   // Should stay null
				"comment": types.StringNull(), // Should stay null
			},
		)
		entry2 := types.ObjectValueMust(
			entryAttrTypes,
			map[string]attr.Value{
				"id":      types.StringNull(),
				"ip":      types.StringValue("10.0.0.2"),
				"subnet":  types.Int64Null(),
				"negated": types.BoolValue(true),                 // Should keep true
				"comment": types.StringValue("explicit comment"), // Should keep value
			},
		)
		plannedEntries := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{entry1, entry2})

		var diags diag.Diagnostics
		result := flattenEntries(ctx, remote, plannedEntries, &diags)

		assert.False(t, diags.HasError())
		elements := result.Elements()
		assert.Equal(t, 2, len(elements))

		// Find entries by IP (set order may vary)
		var attrs1, attrs2 map[string]attr.Value
		for _, elem := range elements {
			entry := elem.(types.Object)
			attrs := entry.Attributes()
			ip := attrs["ip"].(types.String).ValueString()
			switch ip {
			case "10.0.0.1":
				attrs1 = attrs
			case "10.0.0.2":
				attrs2 = attrs
			}
		}

		// Entry 1 should have nulls preserved
		assert.True(t, attrs1["negated"].(types.Bool).IsNull())
		assert.True(t, attrs1["comment"].(types.String).IsNull())

		// Entry 2 should have explicit values
		assert.False(t, attrs2["negated"].(types.Bool).IsNull())
		assert.True(t, attrs2["negated"].(types.Bool).ValueBool())
		assert.Equal(t, "explicit comment", attrs2["comment"].(types.String).ValueString())
	})

	t.Run("handles entry not in plan (new from API)", func(t *testing.T) {
		// API returns an entry that wasn't in the plan (shouldn't happen in normal flow, but be defensive)
		remote := []*fastly.ACLEntry{
			{
				EntryID: new("entry1"),
				IP:      new("10.0.0.1"),
				Negated: new(false),
			},
			{
				EntryID: new("entry2"),
				IP:      new("10.0.0.2"), // Not in plan
				Negated: new(true),
			},
		}

		// Plan only has entry1
		entry1 := types.ObjectValueMust(
			entryAttrTypes,
			map[string]attr.Value{
				"id":      types.StringNull(),
				"ip":      types.StringValue("10.0.0.1"),
				"subnet":  types.Int64Null(),
				"negated": types.BoolNull(),
				"comment": types.StringNull(),
			},
		)
		plannedEntries := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{entry1})

		var diags diag.Diagnostics
		result := flattenEntries(ctx, remote, plannedEntries, &diags)

		assert.False(t, diags.HasError())
		elements := result.Elements()
		assert.Equal(t, 2, len(elements))

		// Entry 1 should preserve nulls, entry 2 should use API values (no plan to preserve from)
		for _, elem := range elements {
			entry := elem.(types.Object)
			attrs := entry.Attributes()
			ip := attrs["ip"].(types.String).ValueString()

			switch ip {
			case "10.0.0.1":
				assert.True(t, attrs["negated"].(types.Bool).IsNull())
			case "10.0.0.2":
				// Should use API value since not in plan
				assert.False(t, attrs["negated"].(types.Bool).IsNull())
				assert.True(t, attrs["negated"].(types.Bool).ValueBool())
			}
		}
	})
}

func TestExpandEntries(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		input    types.Set
		wantLen  int
		wantErrs bool
	}{
		{
			name:     "null set",
			input:    types.SetNull(types.ObjectType{AttrTypes: entryAttrTypes}),
			wantLen:  0,
			wantErrs: false,
		},
		{
			name:     "unknown set",
			input:    types.SetUnknown(types.ObjectType{AttrTypes: entryAttrTypes}),
			wantLen:  0,
			wantErrs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			result := expandEntries(ctx, tt.input, &diags)

			assert.Equal(t, tt.wantErrs, diags.HasError())
			assert.Equal(t, tt.wantLen, len(result))
		})
	}
}

func TestBuildBatchACLEntry(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		entry EntryModel
		op    fastly.BatchOperation
		want  *fastly.BatchACLEntry
	}{
		{
			name: "complete entry",
			entry: EntryModel{
				ID:      types.StringValue("entry123"),
				IP:      types.StringValue("10.0.0.1"),
				Subnet:  types.Int64Value(24),
				Negated: types.BoolValue(true),
				Comment: types.StringValue("test comment"),
			},
			op: fastly.CreateBatchOperation,
			want: &fastly.BatchACLEntry{
				Operation: new(fastly.CreateBatchOperation),
				EntryID:   new("entry123"),
				IP:        new("10.0.0.1"),
				Subnet:    new(24),
				Negated:   new(fastly.Compatibool(true)),
				Comment:   new("test comment"),
			},
		},
		{
			name: "minimal entry",
			entry: EntryModel{
				IP:      types.StringValue("192.168.1.1"),
				Negated: types.BoolValue(false),
			},
			op: fastly.CreateBatchOperation,
			want: &fastly.BatchACLEntry{
				Operation: new(fastly.CreateBatchOperation),
				IP:        new("192.168.1.1"),
				Negated:   new(fastly.Compatibool(false)),
			},
		},
		{
			name: "zero subnet",
			entry: EntryModel{
				IP:      types.StringValue("1.2.3.4"),
				Subnet:  types.Int64Value(0),
				Negated: types.BoolValue(false),
			},
			op: fastly.UpdateBatchOperation,
			want: &fastly.BatchACLEntry{
				Operation: new(fastly.UpdateBatchOperation),
				IP:        new("1.2.3.4"),
				Subnet:    new(0),
				Negated:   new(fastly.Compatibool(false)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildBatchACLEntry(ctx, tt.entry, tt.op)

			assert.NotNil(t, got.Operation)
			assert.Equal(t, *tt.want.Operation, *got.Operation)

			if tt.want.EntryID != nil {
				assert.NotNil(t, got.EntryID)
				assert.Equal(t, *tt.want.EntryID, *got.EntryID)
			}

			if tt.want.IP != nil {
				assert.NotNil(t, got.IP)
				assert.Equal(t, *tt.want.IP, *got.IP)
			}

			if tt.want.Subnet != nil {
				assert.NotNil(t, got.Subnet)
				assert.Equal(t, *tt.want.Subnet, *got.Subnet)
			}

			if tt.want.Negated != nil {
				assert.NotNil(t, got.Negated)
				assert.Equal(t, *tt.want.Negated, *got.Negated)
			}

			if tt.want.Comment != nil {
				assert.NotNil(t, got.Comment)
				assert.Equal(t, *tt.want.Comment, *got.Comment)
			}
		})
	}
}

func TestEntryAttrTypes(t *testing.T) {
	attrs := entryAttrTypes

	assert.Equal(t, 5, len(attrs))
	assert.Equal(t, types.StringType, attrs["id"])
	assert.Equal(t, types.StringType, attrs["ip"])
	assert.Equal(t, types.Int64Type, attrs["subnet"])
	assert.Equal(t, types.BoolType, attrs["negated"])
	assert.Equal(t, types.StringType, attrs["comment"])
}

// TestFlattenEntries_SameIPDifferentSubnet exercises the key-collision fix: two
// entries sharing the same IP but different subnets must not overwrite each
// other in the planned-entries lookup map.
func TestFlattenEntries_SameIPDifferentSubnet(t *testing.T) {
	ctx := context.Background()

	remote := []*fastly.ACLEntry{
		{EntryID: new("e1"), IP: new("10.0.0.1"), Subnet: new(24), Negated: new(false), Comment: new("")},
		{EntryID: new("e2"), IP: new("10.0.0.1"), Subnet: new(32), Negated: new(false), Comment: new("")},
	}

	entry24 := types.ObjectValueMust(entryAttrTypes, map[string]attr.Value{
		"id": types.StringNull(), "ip": types.StringValue("10.0.0.1"),
		"subnet": types.Int64Value(24), "negated": types.BoolValue(false), "comment": types.StringNull(),
	})
	entry32 := types.ObjectValueMust(entryAttrTypes, map[string]attr.Value{
		"id": types.StringNull(), "ip": types.StringValue("10.0.0.1"),
		"subnet": types.Int64Value(32), "negated": types.BoolValue(false), "comment": types.StringNull(),
	})
	planned := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{entry24, entry32})

	var diags diag.Diagnostics
	result := flattenEntries(ctx, remote, planned, &diags)

	assert.False(t, diags.HasError())
	assert.Equal(t, 2, len(result.Elements()))

	subnets := make(map[int64]bool)
	for _, elem := range result.Elements() {
		obj := elem.(types.Object)
		subnets[obj.Attributes()["subnet"].(types.Int64).ValueInt64()] = true
		// comment was null in plan — both entries must preserve that null
		assert.True(t, obj.Attributes()["comment"].(types.String).IsNull(),
			"comment should be null when planned as null")
	}
	assert.True(t, subnets[24], "subnet /24 entry should be present")
	assert.True(t, subnets[32], "subnet /32 entry should be present")
}

// TestPreserveEntryIDsModifier_PopulatesKnownIDs checks that entry IDs from
// state are copied into the plan when the entry content matches.
func TestPreserveEntryIDsModifier_PopulatesKnownIDs(t *testing.T) {
	ctx := context.Background()

	stateEntry := types.ObjectValueMust(entryAttrTypes, map[string]attr.Value{
		"id": types.StringValue("known-id"), "ip": types.StringValue("10.0.0.1"),
		"subnet": types.Int64Value(24), "negated": types.BoolValue(false), "comment": types.StringValue("c"),
	})
	stateSet := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{stateEntry})

	// Plan has the same entry but ID is unknown (as Terraform would produce).
	planEntry := types.ObjectValueMust(entryAttrTypes, map[string]attr.Value{
		"id": types.StringUnknown(), "ip": types.StringValue("10.0.0.1"),
		"subnet": types.Int64Value(24), "negated": types.BoolValue(false), "comment": types.StringValue("c"),
	})
	planSet := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{planEntry})

	req := planmodifier.SetRequest{StateValue: stateSet, PlanValue: planSet}
	resp := &planmodifier.SetResponse{PlanValue: planSet}

	m := preserveEntryIDsModifier{}
	m.PlanModifySet(ctx, req, resp)

	assert.False(t, resp.Diagnostics.HasError())
	assert.Equal(t, 1, len(resp.PlanValue.Elements()))
	obj := resp.PlanValue.Elements()[0].(types.Object)
	assert.Equal(t, "known-id", obj.Attributes()["id"].(types.String).ValueString())
}

// TestPreserveEntryIDsModifier_NullStateNoOp checks that a null state (new
// resource) leaves the plan unchanged.
func TestPreserveEntryIDsModifier_NullStateNoOp(t *testing.T) {
	ctx := context.Background()

	planEntry := types.ObjectValueMust(entryAttrTypes, map[string]attr.Value{
		"id": types.StringUnknown(), "ip": types.StringValue("10.0.0.1"),
		"subnet": types.Int64Null(), "negated": types.BoolNull(), "comment": types.StringNull(),
	})
	planSet := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{planEntry})

	req := planmodifier.SetRequest{
		StateValue: types.SetNull(types.ObjectType{AttrTypes: entryAttrTypes}),
		PlanValue:  planSet,
	}
	resp := &planmodifier.SetResponse{PlanValue: planSet}

	m := preserveEntryIDsModifier{}
	m.PlanModifySet(ctx, req, resp)

	assert.False(t, resp.Diagnostics.HasError())
	// PlanValue must be unchanged — ID remains unknown.
	assert.Equal(t, planSet, resp.PlanValue)
}

// TestPreserveEntryIDsModifier_ContentMismatchNoIDCopied checks that when
// planned entry content differs from state, no ID is copied.
func TestPreserveEntryIDsModifier_ContentMismatchNoIDCopied(t *testing.T) {
	ctx := context.Background()

	stateEntry := types.ObjectValueMust(entryAttrTypes, map[string]attr.Value{
		"id": types.StringValue("old-id"), "ip": types.StringValue("10.0.0.1"),
		"subnet": types.Int64Value(24), "negated": types.BoolValue(false), "comment": types.StringValue(""),
	})
	stateSet := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{stateEntry})

	// Plan has a different IP — should not receive the old ID.
	planEntry := types.ObjectValueMust(entryAttrTypes, map[string]attr.Value{
		"id": types.StringUnknown(), "ip": types.StringValue("192.168.1.1"),
		"subnet": types.Int64Value(24), "negated": types.BoolValue(false), "comment": types.StringValue(""),
	})
	planSet := types.SetValueMust(types.ObjectType{AttrTypes: entryAttrTypes}, []attr.Value{planEntry})

	req := planmodifier.SetRequest{StateValue: stateSet, PlanValue: planSet}
	resp := &planmodifier.SetResponse{PlanValue: planSet}

	m := preserveEntryIDsModifier{}
	m.PlanModifySet(ctx, req, resp)

	assert.False(t, resp.Diagnostics.HasError())
	obj := resp.PlanValue.Elements()[0].(types.Object)
	assert.True(t, obj.Attributes()["id"].(types.String).IsUnknown(),
		"ID should remain unknown when content does not match any state entry")
}

// TestPlannedEntryContentKey checks that the content key function produces
// stable, distinct keys for entries differing only in subnet.
func TestPlannedEntryContentKey(t *testing.T) {
	e24 := EntryModel{
		IP: types.StringValue("10.0.0.1"), Subnet: types.Int64Value(24),
		Negated: types.BoolValue(false), Comment: types.StringValue(""),
	}
	e32 := EntryModel{
		IP: types.StringValue("10.0.0.1"), Subnet: types.Int64Value(32),
		Negated: types.BoolValue(false), Comment: types.StringValue(""),
	}

	assert.NotEqual(t, plannedEntryContentKey(e24), plannedEntryContentKey(e32),
		"entries with same IP but different subnet must produce different keys")
	assert.Equal(t, plannedEntryContentKey(e24), plannedEntryContentKey(e24),
		"same entry must produce identical key on repeated calls")
}

// TestEntryIdentityKey_IgnoresNegatedAndComment guards against regressing to a
// content-based diff key in Update(): two entries with the same ip/subnet but
// different negated/comment must produce the SAME identity key, so a
// comment-or-negated-only change is diffed as an update to the existing entry
// rather than a delete-and-create pair at the same ip/subnet (which Fastly's
// batch ACL entries API rejects as a duplicate).
func TestEntryIdentityKey_IgnoresNegatedAndComment(t *testing.T) {
	original := EntryModel{
		IP: types.StringValue("192.168.1.0"), Subnet: types.Int64Value(24),
		Negated: types.BoolValue(false), Comment: types.StringValue("original"),
	}
	commentChanged := EntryModel{
		IP: types.StringValue("192.168.1.0"), Subnet: types.Int64Value(24),
		Negated: types.BoolValue(false), Comment: types.StringValue("updated"),
	}
	negatedChanged := EntryModel{
		IP: types.StringValue("192.168.1.0"), Subnet: types.Int64Value(24),
		Negated: types.BoolValue(true), Comment: types.StringValue("original"),
	}
	differentSubnet := EntryModel{
		IP: types.StringValue("192.168.1.0"), Subnet: types.Int64Value(32),
		Negated: types.BoolValue(false), Comment: types.StringValue("original"),
	}
	differentIP := EntryModel{
		IP: types.StringValue("192.168.2.0"), Subnet: types.Int64Value(24),
		Negated: types.BoolValue(false), Comment: types.StringValue("original"),
	}

	assert.Equal(t, entryIdentityKey(original), entryIdentityKey(commentChanged),
		"comment-only change must keep the same identity key")
	assert.Equal(t, entryIdentityKey(original), entryIdentityKey(negatedChanged),
		"negated-only change must keep the same identity key")
	assert.NotEqual(t, entryIdentityKey(original), entryIdentityKey(differentSubnet),
		"different subnet must produce a different identity key")
	assert.NotEqual(t, entryIdentityKey(original), entryIdentityKey(differentIP),
		"different IP must produce a different identity key")
}

// TestFilterManagedRemoteEntries checks that only remote entries whose
// identity is present in managed are returned -- entries present remotely
// but never declared by this resource must be left out, so they are
// preserved and ignored for drift.
func TestFilterManagedRemoteEntries(t *testing.T) {
	remote := []*fastly.ACLEntry{
		{EntryID: new("e1"), IP: new("10.0.0.1"), Subnet: new(24), Negated: new(false), Comment: new("managed")},
		{EntryID: new("e2"), IP: new("192.168.0.1"), Subnet: new(32), Negated: new(false), Comment: new("external")},
	}
	managed := []EntryModel{
		{IP: types.StringValue("10.0.0.1"), Subnet: types.Int64Value(24)},
		{IP: types.StringValue("172.16.0.1"), Subnet: types.Int64Value(16)}, // deleted externally, absent from remote
	}

	got := filterManagedRemoteEntries(remote, managed)

	assert.Len(t, got, 1)
	assert.Equal(t, "e1", *got[0].EntryID)
}

// TestBuildBatchEntries_CreatesMissingEntries checks that desired entries
// absent from remote produce Create operations.
func TestBuildBatchEntries_CreatesMissingEntries(t *testing.T) {
	ctx := context.Background()
	desired := []EntryModel{
		{IP: types.StringValue("10.0.0.1"), Subnet: types.Int64Value(24), Negated: types.BoolValue(false), Comment: types.StringValue("new")},
	}

	batch := buildBatchEntries(ctx, nil, nil, desired)

	assert.Len(t, batch, 1)
	assert.Equal(t, fastly.CreateBatchOperation, *batch[0].Operation)
	assert.Equal(t, "10.0.0.1", *batch[0].IP)
}

// TestBuildBatchEntries_AdoptsExistingRemoteEntry checks that when Create
// declares an entry that already exists remotely at the same identity (e.g.
// created outside Terraform, or left over from a prior partial apply), the
// entry is adopted via an Update rather than re-created, since the batch API
// rejects a Create colliding with an existing ip/subnet.
func TestBuildBatchEntries_AdoptsExistingRemoteEntry(t *testing.T) {
	ctx := context.Background()
	remote := []*fastly.ACLEntry{
		{EntryID: new("existing"), IP: new("10.0.0.1"), Subnet: new(24), Negated: new(false), Comment: new("old comment")},
	}
	desired := []EntryModel{
		{IP: types.StringValue("10.0.0.1"), Subnet: types.Int64Value(24), Negated: types.BoolValue(false), Comment: types.StringValue("new comment")},
	}

	batch := buildBatchEntries(ctx, remote, nil, desired)

	assert.Len(t, batch, 1)
	assert.Equal(t, fastly.UpdateBatchOperation, *batch[0].Operation)
	assert.Equal(t, "existing", *batch[0].EntryID)
	assert.Equal(t, "new comment", *batch[0].Comment)
}

// TestBuildBatchEntries_NoOpWhenRemoteAlreadyMatches checks that no batch
// operation is produced when the remote entry already matches the desired
// content.
func TestBuildBatchEntries_NoOpWhenRemoteAlreadyMatches(t *testing.T) {
	ctx := context.Background()
	remote := []*fastly.ACLEntry{
		{EntryID: new("e1"), IP: new("10.0.0.1"), Subnet: new(24), Negated: new(false), Comment: new("same")},
	}
	desired := []EntryModel{
		{IP: types.StringValue("10.0.0.1"), Subnet: types.Int64Value(24), Negated: types.BoolValue(false), Comment: types.StringValue("same")},
	}

	batch := buildBatchEntries(ctx, remote, desired, desired)

	assert.Empty(t, batch)
}

// TestBuildBatchEntries_DeletesOnlyManagedAndRemoved checks that only entries
// that were previously managed AND removed from desired are deleted, and only
// when they still exist remotely -- unmanaged remote entries are left alone.
func TestBuildBatchEntries_DeletesOnlyManagedAndRemoved(t *testing.T) {
	ctx := context.Background()
	remote := []*fastly.ACLEntry{
		{EntryID: new("managed-removed"), IP: new("10.0.0.1"), Subnet: new(24), Negated: new(false)},
		{EntryID: new("external"), IP: new("192.168.0.1"), Subnet: new(32), Negated: new(false)},
	}
	currentManaged := []EntryModel{
		{IP: types.StringValue("10.0.0.1"), Subnet: types.Int64Value(24)},
	}
	var desired []EntryModel // entry removed from config

	batch := buildBatchEntries(ctx, remote, currentManaged, desired)

	assert.Len(t, batch, 1)
	assert.Equal(t, fastly.DeleteBatchOperation, *batch[0].Operation)
	assert.Equal(t, "managed-removed", *batch[0].EntryID)
}

// TestBuildBatchEntries_PreservesExternalEntries checks that remote entries
// outside currentManaged and desired never appear in the batch, whether or
// not they happen to share an identity that this resource cares about.
func TestBuildBatchEntries_PreservesExternalEntries(t *testing.T) {
	ctx := context.Background()
	remote := []*fastly.ACLEntry{
		{EntryID: new("external-1"), IP: new("203.0.113.1"), Subnet: new(32), Negated: new(false), Comment: new("outside terraform")},
	}

	batch := buildBatchEntries(ctx, remote, nil, nil)

	assert.Empty(t, batch)
}

// TestRemoteEntryIdentityKey_MatchesEntryIdentityKey checks that the API-
// sourced and config/state-sourced identity key builders agree for equal
// values, since buildBatchEntries and filterManagedRemoteEntries rely on
// matching keys across both.
func TestRemoteEntryIdentityKey_MatchesEntryIdentityKey(t *testing.T) {
	remote := &fastly.ACLEntry{IP: new("10.0.0.1"), Subnet: new(24)}
	model := EntryModel{IP: types.StringValue("10.0.0.1"), Subnet: types.Int64Value(24)}

	assert.Equal(t, entryIdentityKey(model), remoteEntryIdentityKey(remote))
}
