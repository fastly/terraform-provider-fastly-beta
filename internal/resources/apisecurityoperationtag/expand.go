package apisecurityoperationtag

import (
	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"
)

func buildCreateInput(serviceID string, plan Model) *operations.CreateTagInput {
	in := &operations.CreateTagInput{
		ServiceID: &serviceID,
		Name:      plan.Name.ValueStringPointer(),
	}

	if !plan.Description.IsNull() {
		in.Description = plan.Description.ValueStringPointer()
	}

	return in
}

// Always sends name and description: PATCH here doesn't merge, so an
// omitted field gets cleared rather than left alone.
func buildUpdateInput(serviceID, tagID string, plan Model) *operations.UpdateTagInput {
	description := plan.Description.ValueString()

	return &operations.UpdateTagInput{
		ServiceID:   &serviceID,
		TagID:       &tagID,
		Name:        plan.Name.ValueStringPointer(),
		Description: &description,
	}
}
