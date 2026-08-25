package apisecurityoperation

import (
	"context"
	"strings"

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
	method := strings.ToUpper(plan.Method.ValueString())

	in := &operations.CreateInput{
		ServiceID: &serviceID,
		Method:    &method,
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

// buildUpdateInput always sends description and tag_ids, even when unchanged:
// the API's PATCH doesn't merge partial input, so an omitted field gets
// cleared rather than left alone. TagIDs still can't be cleared to empty on
// the wire (its `omitempty` tag drops empty slices), but a non-empty value
// now survives an update that only touches description.
func buildUpdateInput(ctx context.Context, serviceID, operationID string, plan Model) *operations.UpdateInput {
	description := plan.Description.ValueString()

	return &operations.UpdateInput{
		ServiceID:   &serviceID,
		OperationID: &operationID,
		Description: &description,
		TagIDs:      expandTagIDs(ctx, plan.TagIDs),
	}
}
