package ngwaflist

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

// FlattenToModel converts an API list response to Terraform state and verifies
// that the list belongs to the concrete workspace list resource type.
func FlattenToModel(listType, workspaceID string, list *lists.List) (Model, error) {
	if list == nil {
		return Model{}, fmt.Errorf("cannot flatten nil NGWAF workspace list")
	}
	if list.ListID == "" {
		return Model{}, fmt.Errorf("invalid NGWAF workspace list: id is missing")
	}
	if list.Type != listType {
		return Model{}, fmt.Errorf("list %s is a %q list, not %q; use the resource for %q lists instead", list.ListID, list.Type, listType, list.Type)
	}
	if list.Scope.Type == "" {
		return Model{}, fmt.Errorf("invalid NGWAF list scope: type is missing")
	}
	if scope.Type(list.Scope.Type) != scope.ScopeTypeWorkspace {
		return Model{}, fmt.Errorf("expected workspace-scoped NGWAF list, got scope type %q", list.Scope.Type)
	}

	entries, diags := types.ListValueFrom(context.Background(), types.StringType, list.Entries)
	if diags.HasError() {
		return Model{}, fmt.Errorf("flattening NGWAF workspace list entries: %s", diags[0].Summary())
	}

	return Model{
		ID:          types.StringValue(list.ListID),
		WorkspaceID: types.StringValue(workspaceID),
		Name:        types.StringValue(list.Name),
		Description: types.StringValue(list.Description),
		Entries:     entries,
		ReferenceID: types.StringValue(list.ReferenceID),
	}, nil
}

// FlattenAccountToModel converts an API list response to Terraform state and
// verifies that it belongs to both account scope and the concrete list resource type.
func FlattenAccountToModel(listType string, list *lists.List) (AccountModel, error) {
	if list == nil {
		return AccountModel{}, fmt.Errorf("cannot flatten nil NGWAF account list")
	}
	if list.ListID == "" {
		return AccountModel{}, fmt.Errorf("invalid NGWAF account list: id is missing")
	}
	if list.Type != listType {
		return AccountModel{}, fmt.Errorf("list %s is a %q list, not %q; use the resource for %q lists instead", list.ListID, list.Type, listType, list.Type)
	}
	if list.Scope.Type == "" {
		return AccountModel{}, fmt.Errorf("invalid NGWAF list scope: type is missing")
	}
	if scope.Type(list.Scope.Type) != scope.ScopeTypeAccount {
		return AccountModel{}, fmt.Errorf("expected account-scoped NGWAF list, got scope type %q", list.Scope.Type)
	}

	entries, diags := types.ListValueFrom(context.Background(), types.StringType, list.Entries)
	if diags.HasError() {
		return AccountModel{}, fmt.Errorf("flattening NGWAF account list entries: %s", diags[0].Summary())
	}

	return AccountModel{
		ID:          types.StringValue(list.ListID),
		Name:        types.StringValue(list.Name),
		Description: types.StringValue(list.Description),
		Entries:     entries,
		ReferenceID: types.StringValue(list.ReferenceID),
	}, nil
}
