package aclentries

import (
	"context"
	"fmt"

	"github.com/fastly/terraform-provider-fastly-beta/internal/legacystate"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const legacyComputeACLEntriesTypeName = "fastly_compute_acl_entries"

var _ resource.ResourceWithMoveState = &Resource{}

func legacyComputeACLEntriesSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true},
			"compute_acl_id": schema.StringAttribute{Computed: true},
			"entries":        schema.MapAttribute{Computed: true, ElementType: types.StringType},
			"manage_entries": schema.BoolAttribute{Computed: true},
		},
	}
}

// MoveState moves fastly_compute_acl_entries state from the legacy SDKv2 provider.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	sourceSchema := legacyComputeACLEntriesSchema()
	return []resource.StateMover{{
		SourceSchema: &sourceSchema,
		StateMover:   moveLegacyComputeACLEntriesState,
	}}
}

func moveLegacyComputeACLEntriesState(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if !legacystate.AcceptMove(req, legacyComputeACLEntriesTypeName, "fastly_acl_entries", &resp.Diagnostics) {
		return
	}

	var id, aclID types.String
	var entries types.Map
	var manageEntries types.Bool
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("compute_acl_id"), &aclID)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("entries"), &entries)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("manage_entries"), &manageEntries)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !legacystate.HasValue(aclID) {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly ACL entries state",
			fmt.Sprintf("The %s state does not contain a compute_acl_id.", legacyComputeACLEntriesTypeName),
		)
		return
	}
	// The legacy provider always derived id from compute_acl_id.
	if id.ValueString() != resourceID(aclID.ValueString()) {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly ACL entries state",
			fmt.Sprintf("The %s state id %q does not match compute_acl_id %q, so the ACL identity is ambiguous.",
				legacyComputeACLEntriesTypeName, id.ValueString(), aclID.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(resp.TargetState.Set(ctx, Model{
		ID:      id,
		ACLID:   aclID,
		Entries: legacystate.OwnedEntries(manageEntries, entries),
	})...)
}
