package configstoreitems

import (
	"context"
	"fmt"

	"github.com/fastly/terraform-provider-fastly-beta/internal/legacystate"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const legacyConfigStoreEntriesTypeName = "fastly_configstore_entries"

var _ resource.ResourceWithMoveState = &Resource{}

func legacyConfigStoreEntriesSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true},
			"store_id":       schema.StringAttribute{Computed: true},
			"entries":        schema.MapAttribute{Computed: true, ElementType: types.StringType},
			"manage_entries": schema.BoolAttribute{Computed: true},
		},
	}
}

// MoveState moves fastly_configstore_entries state from the legacy SDKv2 provider.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	sourceSchema := legacyConfigStoreEntriesSchema()
	return []resource.StateMover{{
		SourceSchema: &sourceSchema,
		StateMover:   moveLegacyConfigStoreEntriesState,
	}}
}

func moveLegacyConfigStoreEntriesState(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if !legacystate.AcceptMove(req, legacyConfigStoreEntriesTypeName, "fastly_configstore_items", &resp.Diagnostics) {
		return
	}

	var id, storeID types.String
	var entries types.Map
	var manageEntries types.Bool
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("store_id"), &storeID)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("entries"), &entries)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("manage_entries"), &manageEntries)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !legacystate.HasValue(storeID) {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly Config Store entries state",
			fmt.Sprintf("The %s state does not contain a store_id.", legacyConfigStoreEntriesTypeName),
		)
		return
	}
	// The legacy id was <store_id>/entries; the target uses <store_id>/items.
	if id.ValueString() != storeID.ValueString()+"/entries" {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly Config Store entries state",
			fmt.Sprintf("The %s state id %q does not match store_id %q, so the Config Store identity is ambiguous.",
				legacyConfigStoreEntriesTypeName, id.ValueString(), storeID.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(resp.TargetState.Set(ctx, Model{
		ID:      types.StringValue(resourceID(storeID.ValueString())),
		StoreID: storeID,
		Items:   legacystate.OwnedEntries(manageEntries, entries),
	})...)
}
