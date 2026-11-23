package acl

import (
	"context"
	"fmt"

	"github.com/fastly/terraform-provider-fastly-beta/internal/legacystate"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const legacyComputeACLTypeName = "fastly_compute_acl"

var _ resource.ResourceWithMoveState = &Resource{}

func legacyComputeACLSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true},
			"name": schema.StringAttribute{Computed: true},
		},
	}
}

// MoveState moves fastly_compute_acl state from the legacy SDKv2 provider.
func (r *Resource) MoveState(_ context.Context) []resource.StateMover {
	sourceSchema := legacyComputeACLSchema()
	return []resource.StateMover{{
		SourceSchema: &sourceSchema,
		StateMover:   moveLegacyComputeACLState,
	}}
}

func moveLegacyComputeACLState(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
	if !legacystate.AcceptMove(req, legacyComputeACLTypeName, "fastly_acl", &resp.Diagnostics) {
		return
	}

	var id, name types.String
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.SourceState.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, required := range []struct {
		name  string
		value types.String
	}{{"id", id}, {"name", name}} {
		if !legacystate.HasValue(required.value) {
			resp.Diagnostics.AddError(
				"Unable to migrate legacy Fastly ACL state",
				fmt.Sprintf("The %s state does not contain a %s.", legacyComputeACLTypeName, required.name),
			)
			return
		}
	}
	resp.Diagnostics.Append(resp.TargetState.Set(ctx, Model{ID: id, Name: name})...)
}
