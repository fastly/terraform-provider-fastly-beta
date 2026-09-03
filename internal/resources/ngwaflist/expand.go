package ngwaflist

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

func BuildCreateInput(ctx context.Context, listType string, plan CommonModel, listScope *scope.Scope) (*lists.CreateInput, diag.Diagnostics) {
	entries, diags := expandEntries(ctx, plan.Entries)
	if diags.HasError() {
		return nil, diags
	}

	input := &lists.CreateInput{
		Name:    plan.Name.ValueStringPointer(),
		Type:    &listType,
		Entries: &entries,
		Scope:   listScope,
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = plan.Description.ValueStringPointer()
	}

	return input, nil
}

func BuildUpdateInput(ctx context.Context, listID string, plan CommonModel, listScope *scope.Scope) (*lists.UpdateInput, diag.Diagnostics) {
	entries, diags := expandEntries(ctx, plan.Entries)
	if diags.HasError() {
		return nil, diags
	}

	input := &lists.UpdateInput{
		ListID:  &listID,
		Entries: &entries,
		Scope:   listScope,
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = plan.Description.ValueStringPointer()
	}

	return input, nil
}

func BuildGetInput(listID string, listScope *scope.Scope) *lists.GetInput {
	return &lists.GetInput{
		ListID: &listID,
		Scope:  listScope,
	}
}

func BuildDeleteInput(listID string, listScope *scope.Scope) *lists.DeleteInput {
	return &lists.DeleteInput{
		ListID: &listID,
		Scope:  listScope,
	}
}

func expandEntries(ctx context.Context, entries types.List) ([]string, diag.Diagnostics) {
	var values []string
	diags := entries.ElementsAs(ctx, &values, false)
	if diags.HasError() {
		return nil, diags
	}

	return values, nil
}
