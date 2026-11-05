package ngwaflist

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafquery"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/lists"
	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/scope"
)

var (
	_ list.ListResource              = &ListResource{}
	_ list.ListResourceWithConfigure = &ListResource{}
)

// ListResource implements terraform query for one concrete type-specific NGWAF
// list resource.
type ListResource struct {
	client     *fastly.Client
	definition Definition
}

// NewListResource returns the list resource described by definition.
func NewListResource(definition Definition) list.ListResource {
	return &ListResource{definition: definition}
}

// NewWorkspaceListResource returns the list resource for a workspace-scoped
// NGWAF list of one concrete API list type.
func NewWorkspaceListResource(listType, typeSuffix, description string) list.ListResource {
	return NewListResource(Definition{
		ListType:    listType,
		TypeSuffix:  typeSuffix,
		Description: description,
		Scope:       scope.ScopeTypeWorkspace,
	})
}

// NewAccountListResource returns the list resource for an account-scoped NGWAF
// list of one concrete API list type.
func NewAccountListResource(listType, typeSuffix, description string) list.ListResource {
	return NewListResource(Definition{
		ListType:    listType,
		TypeSuffix:  typeSuffix,
		Description: description,
		Scope:       scope.ScopeTypeAccount,
	})
}

func (l *ListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	scopePrefix := ""
	if l.definition.Scope == scope.ScopeTypeWorkspace {
		scopePrefix = "workspace_"
	}
	resp.TypeName = req.ProviderTypeName + "_ngwaf_" + scopePrefix + l.definition.TypeSuffix + "_list"
}

func (l *ListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Description: fmt.Sprintf("List all Fastly Next-Gen WAF `%s` lists defined at %s scope that are accessible to the API token.", l.definition.ListType, l.definition.Scope),
	}
}

func (l *ListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	l.client = data.Client
}

type listEntry struct {
	list        *lists.List
	workspaceID string
}

func (l *ListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Debug(ctx, "Listing Fastly NGWAF lists", map[string]any{
		"type":  l.definition.ListType,
		"scope": l.definition.Scope,
	})

	entries, diags := l.collect(ctx)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		var count int64
		for _, e := range entries {
			if req.Limit > 0 && count >= req.Limit {
				return
			}
			count++

			result := req.NewListResult(ctx)
			result.DisplayName = service.ToGeneratedResourceName(e.list.Name, e.list.ListID)

			var state any
			if l.definition.Scope == scope.ScopeTypeAccount {
				result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.NGWAFAccountObject(types.StringValue(e.list.ListID)))...)
				model, err := FlattenAccountToModel(l.definition.ListType, e.list)
				if err != nil {
					result.Diagnostics.AddError("Error reading NGWAF list", err.Error())
				}
				state = &model
			} else {
				result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.NGWAFWorkspaceObject(types.StringValue(e.workspaceID), types.StringValue(e.list.ListID)))...)
				model, err := FlattenToModel(l.definition.ListType, e.workspaceID, e.list)
				if err != nil {
					result.Diagnostics.AddError("Error reading NGWAF list", err.Error())
				}
				state = &model
			}

			if req.IncludeResource && !result.Diagnostics.HasError() {
				result.Diagnostics.Append(result.Resource.Set(ctx, state)...)
			}

			if !push(result) {
				return
			}
		}
	}
}

func (l *ListResource) collect(ctx context.Context) ([]listEntry, diag.Diagnostics) {
	var diags diag.Diagnostics

	if l.definition.Scope == scope.ScopeTypeAccount {
		found, err := l.listScope(ctx, AccountScope())
		if err != nil {
			diags.AddError("Error listing NGWAF account lists", err.Error())
			return nil, diags
		}
		return wrap(found, ""), diags
	}

	workspaces, diags := ngwafquery.ListWorkspaces(ctx, l.client)
	if diags.HasError() {
		return nil, diags
	}

	var out []listEntry
	for _, w := range workspaces {
		found, err := l.listScope(ctx, WorkspaceScope(w.WorkspaceID))
		if err != nil {
			tflog.Warn(ctx, "Error listing NGWAF lists for workspace", map[string]any{
				"workspace_id": w.WorkspaceID,
				"error":        err.Error(),
			})
			continue
		}
		out = append(out, wrap(found, w.WorkspaceID)...)
	}
	return out, diags
}

// listScope returns the lists of this resource's type that are owned by the
// given scope, since the workspace endpoint can also return lists inherited
// from the account.
func (l *ListResource) listScope(ctx context.Context, apiScope *scope.Scope) ([]*lists.List, error) {
	resp, err := lists.ListLists(ctx, l.client, &lists.ListInput{Scope: apiScope})
	if err != nil {
		return nil, err
	}

	var out []*lists.List
	for i := range resp.Data {
		item := &resp.Data[i]
		if item.ListID == "" || item.Type != l.definition.ListType || scope.Type(item.Scope.Type) != apiScope.Type {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func wrap(found []*lists.List, workspaceID string) []listEntry {
	out := make([]listEntry, 0, len(found))
	for _, item := range found {
		out = append(out, listEntry{list: item, workspaceID: workspaceID})
	}
	return out
}
