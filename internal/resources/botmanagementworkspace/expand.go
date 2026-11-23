package botmanagementworkspace

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/types"

	ws "github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces"
)

// BuildUpdateInput sends only the attributes the plan knows. name and
// description are Optional+Computed, so they're unknown on create when
// omitted, and the API must keep its current values for them.
func BuildUpdateInput(plan Model) *ws.UpdateInput {
	return &ws.UpdateInput{
		WorkspaceID:    new(service.StringValue(plan.WorkspaceID)),
		Name:           knownString(plan.Name),
		Description:    knownString(plan.Description),
		ProtectionMode: knownString(plan.ProtectionMode),
	}
}

func knownString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return new(v.ValueString())
}
