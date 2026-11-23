package cdnaclentries

import (
	"context"
	"fmt"
	"strconv"

	"github.com/fastly/terraform-provider-fastly-beta/internal/legacystate"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const legacyServiceACLEntriesTypeName = "fastly_service_acl_entries"

var _ resource.ResourceWithMoveState = &Resource{}

type legacyEntryModel struct {
	ID      types.String `tfsdk:"id"`
	IP      types.String `tfsdk:"ip"`
	Subnet  types.String `tfsdk:"subnet"`
	Negated types.Bool   `tfsdk:"negated"`
	Comment types.String `tfsdk:"comment"`
}

func legacyServiceACLEntriesSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true},
			"service_id":     schema.StringAttribute{Computed: true},
			"acl_id":         schema.StringAttribute{Computed: true},
			"manage_entries": schema.BoolAttribute{Computed: true},
		},
		Blocks: map[string]schema.Block{
			"entry": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id":      schema.StringAttribute{Computed: true},
						"ip":      schema.StringAttribute{Computed: true},
						"subnet":  schema.StringAttribute{Computed: true},
						"negated": schema.BoolAttribute{Computed: true},
						"comment": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

// MoveState moves fastly_service_acl_entries state from the legacy SDKv2 provider.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	sourceSchema := legacyServiceACLEntriesSchema()
	return []resource.StateMover{{
		SourceSchema: &sourceSchema,
		StateMover:   moveLegacyServiceACLEntriesState,
	}}
}

func moveLegacyServiceACLEntriesState(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if !legacystate.AcceptMove(req, legacyServiceACLEntriesTypeName, "fastly_service_cdn_acl_entries", &resp.Diagnostics) {
		return
	}

	var id, serviceID, aclID types.String
	var manageEntries types.Bool
	var legacyEntries []legacyEntryModel
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("service_id"), &serviceID)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("acl_id"), &aclID)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("manage_entries"), &manageEntries)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("entry"), &legacyEntries)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, required := range []struct {
		name  string
		value types.String
	}{{"service_id", serviceID}, {"acl_id", aclID}} {
		if !legacystate.HasValue(required.value) {
			resp.Diagnostics.AddError(
				"Unable to migrate legacy Fastly ACL entries state",
				fmt.Sprintf("The %s state does not contain a %s.", legacyServiceACLEntriesTypeName, required.name),
			)
			return
		}
	}
	wantID := serviceID.ValueString() + "/" + aclID.ValueString()
	if id.ValueString() != wantID {
		resp.Diagnostics.AddError(
			"Unable to migrate legacy Fastly ACL entries state",
			fmt.Sprintf("The %s state id %q does not match %q, so the ACL identity is ambiguous.",
				legacyServiceACLEntriesTypeName, id.ValueString(), wantID),
		)
		return
	}

	// Without manage_entries the target owns no entries; see legacystate.OwnedEntries.
	elements := []attr.Value{}
	if manageEntries.ValueBool() {
		for _, legacy := range legacyEntries {
			element, ok := convertLegacyEntry(legacy, &resp.Diagnostics)
			if !ok {
				return
			}
			elements = append(elements, element)
		}
	}
	entries, diags := types.SetValue(types.ObjectType{AttrTypes: entryAttrTypes}, elements)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.TargetState.Set(ctx, Model{
		ID:        id,
		ServiceID: serviceID,
		ACLID:     aclID,
		Entry:     entries,
	})...)
	if !resp.Diagnostics.HasError() && resp.TargetIdentity != nil {
		resp.Diagnostics.Append(resp.TargetIdentity.Set(ctx, resourceidentity.ACLCollection(serviceID, aclID))...)
	}
}

// convertLegacyEntry maps one legacy entry; "" subnet or comment becomes null.
// Read matches owned entries by (ip, subnet), so an invalid subnet is rejected.
func convertLegacyEntry(legacy legacyEntryModel, diags *diag.Diagnostics) (attr.Value, bool) {
	if !legacystate.HasValue(legacy.IP) {
		diags.AddError(
			"Unable to migrate legacy Fastly ACL entries state",
			fmt.Sprintf("The %s state contains an entry without an ip.", legacyServiceACLEntriesTypeName),
		)
		return nil, false
	}

	subnet := types.Int64Null()
	if legacystate.HasValue(legacy.Subnet) {
		value, err := strconv.ParseInt(legacy.Subnet.ValueString(), 10, 64)
		if err != nil || value < 0 || value > 128 {
			diags.AddError(
				"Unable to migrate legacy Fastly ACL entries state",
				fmt.Sprintf("The %s entry for ip %q has subnet %q, which is not a prefix length between 0 and 128.",
					legacyServiceACLEntriesTypeName, legacy.IP.ValueString(), legacy.Subnet.ValueString()),
			)
			return nil, false
		}
		subnet = types.Int64Value(value)
	}

	comment := legacy.Comment
	if !legacystate.HasValue(comment) {
		comment = types.StringNull()
	}
	id := legacy.ID
	if !legacystate.HasValue(id) {
		id = types.StringNull()
	}

	value, d := types.ObjectValue(entryAttrTypes, map[string]attr.Value{
		"id":      id,
		"ip":      legacy.IP,
		"subnet":  subnet,
		"negated": legacy.Negated,
		"comment": comment,
	})
	diags.Append(d...)
	return value, !d.HasError()
}
