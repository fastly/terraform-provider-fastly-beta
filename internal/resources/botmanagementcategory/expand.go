package botmanagementcategory

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

func BuildUpdateInput(plan Model) *policy.UpdateCategoryInput {
	return &policy.UpdateCategoryInput{
		WorkspaceID: new(service.StringValue(plan.WorkspaceID)),
		CategoryID:  new(service.StringValue(plan.CategoryID)),
		Action:      &policy.Action{Type: service.StringValue(plan.Action)},
	}
}
