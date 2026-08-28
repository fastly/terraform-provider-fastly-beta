package apisecurityoperation

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"
)

func expandTagIDs(ctx context.Context, s types.Set) []string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}

	var ids []string
	s.ElementsAs(ctx, &ids, false)
	return ids
}

func buildCreateInput(ctx context.Context, serviceID string, plan Model) *operations.CreateInput {
	in := &operations.CreateInput{
		ServiceID: &serviceID,
		Method:    plan.Method.ValueStringPointer(),
		Domain:    plan.Domain.ValueStringPointer(),
		Path:      plan.Path.ValueStringPointer(),
	}

	if !plan.Description.IsNull() {
		in.Description = plan.Description.ValueStringPointer()
	}
	if !plan.TagIDs.IsNull() {
		in.TagIDs = expandTagIDs(ctx, plan.TagIDs)
	}

	return in
}

// Always resends description and tag_ids: PATCH doesn't merge partial input,
// so an omitted field is cleared rather than left alone. Note tag_ids still
// can't be cleared to empty, since its `omitempty` tag drops empty slices.
func buildUpdateInput(ctx context.Context, serviceID, operationID string, plan Model) *operations.UpdateInput {
	description := plan.Description.ValueString()

	return &operations.UpdateInput{
		ServiceID:   &serviceID,
		OperationID: &operationID,
		Description: &description,
		TagIDs:      expandTagIDs(ctx, plan.TagIDs),
	}
}
