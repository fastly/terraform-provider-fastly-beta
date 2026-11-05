package ngwafquery

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v19/fastly"
)

var (
	_ list.ListResource              = &ListResource[struct{}]{}
	_ list.ListResourceWithConfigure = &ListResource[struct{}]{}
)

// ListDefinition describes a list resource for a Next-Gen WAF object of API
// type T that is either account-scoped or lives inside a workspace.
type ListDefinition[T any] struct {
	// TypeName is the full Terraform resource type name, matching the managed
	// resource.
	TypeName string
	// Noun names the object in the generated schema description.
	Noun string
	// Account is true for objects addressed by ID alone. Otherwise objects are
	// discovered by iterating every workspace.
	Account bool
	// List returns the objects owned by a workspace; workspaceID is empty for
	// account-scoped objects.
	List func(ctx context.Context, client *fastly.Client, workspaceID string) ([]T, error)
	// ID returns the object's Fastly-generated ID.
	ID func(item *T) string
	// DisplayName returns the parts used to build the generated resource name.
	DisplayName func(item *T) []string
	// Flatten converts an API object to a pointer to the managed resource's
	// state model.
	Flatten func(ctx context.Context, workspaceID string, item *T) (any, diag.Diagnostics)
}

// ListResource implements terraform query for one Next-Gen WAF resource.
type ListResource[T any] struct {
	client     *fastly.Client
	definition ListDefinition[T]
}

func NewListResource[T any](definition ListDefinition[T]) *ListResource[T] {
	return &ListResource[T]{definition: definition}
}

type listEntry[T any] struct {
	item        *T
	workspaceID string
}

func (l *ListResource[T]) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + l.definition.TypeName
}

func (l *ListResource[T]) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	where := "across all workspaces"
	if l.definition.Account {
		where = "at account scope"
	}
	resp.Schema = listschema.Schema{
		Description: fmt.Sprintf("List all Fastly Next-Gen WAF %s %s that are accessible to the API token.", l.definition.Noun, where),
	}
}

func (l *ListResource[T]) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	l.client = data.Client
}

func (l *ListResource[T]) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Debug(ctx, "Listing Fastly NGWAF objects", map[string]any{"type": l.definition.TypeName})

	entries, diags := l.collect(ctx)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		var count int64
		for _, e := range entries {
			id := l.definition.ID(e.item)
			if id == "" {
				continue
			}
			if req.Limit > 0 && count >= req.Limit {
				return
			}
			count++

			result := req.NewListResult(ctx)
			result.DisplayName = service.ToGeneratedResourceName(append(l.definition.DisplayName(e.item), id)...)

			if l.definition.Account {
				result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.NGWAFAccountObject(types.StringValue(id)))...)
			} else {
				result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.NGWAFWorkspaceObject(types.StringValue(e.workspaceID), types.StringValue(id)))...)
			}

			if req.IncludeResource {
				model, flattenDiags := l.definition.Flatten(ctx, e.workspaceID, e.item)
				result.Diagnostics.Append(flattenDiags...)
				if !flattenDiags.HasError() {
					result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
				}
			}

			if !push(result) {
				return
			}
		}
	}
}

func (l *ListResource[T]) collect(ctx context.Context) ([]listEntry[T], diag.Diagnostics) {
	var diags diag.Diagnostics

	if l.definition.Account {
		items, err := l.definition.List(ctx, l.client, "")
		if err != nil {
			diags.AddError("Error listing NGWAF objects", err.Error())
			return nil, diags
		}
		return wrap(items, ""), diags
	}

	workspaces, diags := ListWorkspaces(ctx, l.client)
	if diags.HasError() {
		return nil, diags
	}

	var out []listEntry[T]
	for _, w := range workspaces {
		items, err := l.definition.List(ctx, l.client, w.WorkspaceID)
		if err != nil {
			tflog.Warn(ctx, "Error listing NGWAF objects for workspace", map[string]any{
				"workspace_id": w.WorkspaceID,
				"type":         l.definition.TypeName,
				"error":        err.Error(),
			})
			continue
		}
		out = append(out, wrap(items, w.WorkspaceID)...)
	}
	return out, diags
}

func wrap[T any](items []T, workspaceID string) []listEntry[T] {
	out := make([]listEntry[T], 0, len(items))
	for i := range items {
		out = append(out, listEntry[T]{item: &items[i], workspaceID: workspaceID})
	}
	return out
}
