package ngwaflist

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

// FlattenToModel converts an API list response to workspace-scoped Terraform state.
func FlattenToModel(listType, workspaceID string, list *lists.List) (Model, error) {
	common, err := flattenCommonModel(listType, scope.ScopeTypeWorkspace, list)
	if err != nil {
		return Model{}, err
	}

	return Model{
		ID:          common.ID,
		WorkspaceID: types.StringValue(workspaceID),
		Name:        common.Name,
		Description: common.Description,
		Entries:     common.Entries,
		ReferenceID: common.ReferenceID,
	}, nil
}

// FlattenAccountToModel converts an API list response to account-scoped Terraform state.
func FlattenAccountToModel(listType string, list *lists.List) (AccountModel, error) {
	common, err := flattenCommonModel(listType, scope.ScopeTypeAccount, list)
	if err != nil {
		return AccountModel{}, err
	}

	return AccountModel(common), nil
}

func flattenCommonModel(listType string, expectedScope scope.Type, list *lists.List) (CommonModel, error) {
	if list == nil {
		return CommonModel{}, fmt.Errorf("cannot flatten nil NGWAF list")
	}
	if list.ListID == "" {
		return CommonModel{}, fmt.Errorf("invalid NGWAF list: id is missing")
	}
	if list.Type != listType {
		return CommonModel{}, fmt.Errorf(
			"list %s is a %q list, not %q; use the resource for %q lists instead",
			list.ListID,
			list.Type,
			listType,
			list.Type,
		)
	}
	if list.Scope.Type == "" {
		return CommonModel{}, fmt.Errorf("invalid NGWAF list scope: type is missing")
	}
	if scope.Type(list.Scope.Type) != expectedScope {
		return CommonModel{}, fmt.Errorf(
			"expected %s-scoped NGWAF list, got scope type %q",
			expectedScope,
			list.Scope.Type,
		)
	}

	entries, diags := types.ListValueFrom(context.Background(), types.StringType, list.Entries)
	if diags.HasError() {
		return CommonModel{}, fmt.Errorf("flattening NGWAF list entries: %s", diags[0].Summary())
	}

	return CommonModel{
		ID:          types.StringValue(list.ListID),
		Name:        types.StringValue(list.Name),
		Description: types.StringValue(list.Description),
		Entries:     entries,
		ReferenceID: types.StringValue(list.ReferenceID),
	}, nil
}
