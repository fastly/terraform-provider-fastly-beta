package ngwafworkspacevirtualpatch

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	vp "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/virtualpatches"
)

func BuildGetInput(workspaceID, virtualPatchID string) *vp.GetInput {
	return &vp.GetInput{
		WorkspaceID:    &workspaceID,
		VirtualPatchID: &virtualPatchID,
	}
}

func BuildUpdateInput(workspaceID, virtualPatchID string, plan Model) *vp.UpdateInput {
	mode := service.StringValue(plan.Mode)
	enabled := service.BoolValue(plan.Enabled)

	return &vp.UpdateInput{
		WorkspaceID:    &workspaceID,
		VirtualPatchID: &virtualPatchID,
		Mode:           &mode,
		Enabled:        &enabled,
	}
}

func BuildDisableInput(workspaceID, virtualPatchID, action string) *vp.UpdateInput {
	enabled := false

	return &vp.UpdateInput{
		WorkspaceID:    &workspaceID,
		VirtualPatchID: &virtualPatchID,
		Mode:           &action,
		Enabled:        &enabled,
	}
}
