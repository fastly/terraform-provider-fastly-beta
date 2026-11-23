package botmanagementcategory

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"
	ws "github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces"
	"github.com/fastly/go-fastly/v19/fastly/botmanagement/v1/workspaces/policy"
)

var (
	_ list.ListResource              = &ListResource{}
	_ list.ListResourceWithConfigure = &ListResource{}
)

type ListResource struct {
	client *fastly.Client
}

func NewListResource() list.ListResource {
	return &ListResource{}
}

type listEntry struct {
	workspaceID string
	category    *policy.Category
}

func (l *ListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bot_management_category"
}

func (l *ListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Description: "List all bot categories across all Fastly Bot Management workspaces accessible to the API token.",
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

func (l *ListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Debug(ctx, "Listing Fastly Bot Management categories")

	entries, diags := l.collect(ctx)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for i, e := range entries {
			if req.Limit > 0 && int64(i) >= req.Limit {
				return
			}

			result := req.NewListResult(ctx)
			result.DisplayName = service.ToGeneratedResourceName(e.category.CategoryID, e.workspaceID)
			result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.BotManagementCategory(
				types.StringValue(e.workspaceID),
				types.StringValue(e.category.CategoryID),
			))...)

			if req.IncludeResource {
				model := FlattenToModel(e.workspaceID, e.category)
				result.Diagnostics.Append(result.Resource.Set(ctx, &model)...)
			}

			if !push(result) {
				return
			}
		}
	}
}

func (l *ListResource) collect(ctx context.Context) ([]listEntry, diag.Diagnostics) {
	var diags diag.Diagnostics

	workspaces, err := ws.List(ctx, l.client, &ws.ListInput{})
	if err != nil {
		diags.AddError("Error listing Bot Management workspaces", err.Error())
		return nil, diags
	}

	var out []listEntry
	for _, w := range workspaces {
		if w.ID == "" {
			continue
		}
		categories, err := policy.ListCategories(ctx, l.client, &policy.ListCategoriesInput{WorkspaceID: &w.ID})
		if err != nil {
			tflog.Warn(ctx, "Error listing Bot Management categories for workspace", map[string]any{"workspace_id": w.ID, "error": err.Error()})
			continue
		}
		for i := range categories {
			if categories[i].CategoryID == "" {
				continue
			}
			out = append(out, listEntry{workspaceID: w.ID, category: &categories[i]})
		}
	}
	return out, diags
}
