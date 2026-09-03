package ngwaflist

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
)

func BuildCreateInput(ctx context.Context, listType string, plan Model) (*lists.CreateInput, diag.Diagnostics) {
	entries, diags := expandEntries(ctx, plan.Entries)
	if diags.HasError() {
		return nil, diags
	}

	input := &lists.CreateInput{
		Name:    plan.Name.ValueStringPointer(),
		Type:    &listType,
		Entries: &entries,
		Scope:   WorkspaceScope(plan.WorkspaceID.ValueString()),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = plan.Description.ValueStringPointer()
	}

	return input, nil
}

func BuildUpdateInput(ctx context.Context, listID string, plan Model) (*lists.UpdateInput, diag.Diagnostics) {
	entries, diags := expandEntries(ctx, plan.Entries)
	if diags.HasError() {
		return nil, diags
	}

	input := &lists.UpdateInput{
		ListID:  &listID,
		Entries: &entries,
		Scope:   WorkspaceScope(plan.WorkspaceID.ValueString()),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = plan.Description.ValueStringPointer()
	}

	return input, nil
}

func BuildGetInput(workspaceID, listID string) *lists.GetInput {
	return &lists.GetInput{
		ListID: &listID,
		Scope:  WorkspaceScope(workspaceID),
	}
}

func BuildDeleteInput(workspaceID, listID string) *lists.DeleteInput {
	return &lists.DeleteInput{
		ListID: &listID,
		Scope:  WorkspaceScope(workspaceID),
	}
}

func BuildAccountCreateInput(ctx context.Context, listType string, plan AccountModel) (*lists.CreateInput, diag.Diagnostics) {
	entries, diags := expandEntries(ctx, plan.Entries)
	if diags.HasError() {
		return nil, diags
	}

	input := &lists.CreateInput{
		Name:    plan.Name.ValueStringPointer(),
		Type:    &listType,
		Entries: &entries,
		Scope:   AccountScope(),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = plan.Description.ValueStringPointer()
	}

	return input, nil
}

func BuildAccountUpdateInput(ctx context.Context, listID string, plan AccountModel) (*lists.UpdateInput, diag.Diagnostics) {
	entries, diags := expandEntries(ctx, plan.Entries)
	if diags.HasError() {
		return nil, diags
	}

	input := &lists.UpdateInput{
		ListID:  &listID,
		Entries: &entries,
		Scope:   AccountScope(),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input.Description = plan.Description.ValueStringPointer()
	}

	return input, nil
}

func BuildAccountGetInput(listID string) *lists.GetInput {
	return &lists.GetInput{
		ListID: &listID,
		Scope:  AccountScope(),
	}
}

func BuildAccountDeleteInput(listID string) *lists.DeleteInput {
	return &lists.DeleteInput{
		ListID: &listID,
		Scope:  AccountScope(),
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
