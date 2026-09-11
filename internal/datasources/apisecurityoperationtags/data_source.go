package apisecurityoperationtags

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

var _ datasource.DataSource = &DataSource{}

// pageLimit is the page size used to walk every page of results before
// returning them here; go-fastly paginates internally on our behalf.
const pageLimit = 100

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	ServiceID types.String `tfsdk:"service_id"`
	Tags      types.List   `tfsdk:"tags"`
	Total     types.Int64  `tfsdk:"total"`
}

var tagAttrTypes = map[string]attr.Type{
	"created_at":      types.StringType,
	"description":     types.StringType,
	"id":              types.StringType,
	"name":            types.StringType,
	"operation_count": types.Int64Type,
	"updated_at":      types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_security_operation_tags"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to list API Security operation tags for a service.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"service_id": schema.StringAttribute{
				Required:    true,
				Description: "Service ID.",
			},
			"tags": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Operation tags.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "Created timestamp (when present).",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "Tag description (when present).",
						},
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Tag ID.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Tag name.",
						},
						"operation_count": schema.Int64Attribute{
							Computed:    true,
							Description: "Number of operations associated with this tag (when present).",
						},
						"updated_at": schema.StringAttribute{
							Computed:    true,
							Description: "Updated timestamp (when present).",
						},
					},
				},
			},
			"total": schema.Int64Attribute{
				Computed:    true,
				Description: "Total number of matching results, as returned by the API.",
			},
		},
	}
}

func (d *DataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	d.client = data.Client
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := service.StringValue(state.ServiceID)

	tflog.Debug(ctx, "Reading Fastly API Security operation tags", map[string]any{"service_id": serviceID})

	page := 0
	limit := pageLimit
	in := &operations.ListTagsInput{
		ServiceID: &serviceID,
		Page:      &page,
		Limit:     &limit,
	}

	// The first page carries meta.total, which the API may compute from a
	// different (e.g. cached or eventually-consistent) count than the number of
	// items ListTagsAll ends up actually returning; fall back to that count
	// only if the API doesn't report one.
	first, err := operations.ListTags(ctx, d.client, in)
	if err != nil {
		resp.Diagnostics.AddError("Error listing API Security operation tags", err.Error())
		return
	}

	all, err := operations.ListTagsAll(ctx, d.client, in)
	if err != nil {
		resp.Diagnostics.AddError("Error listing API Security operation tags", err.Error())
		return
	}

	listVal, listDiags := flattenTags(all)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	total := first.Meta.Total
	if total == 0 {
		total = len(all)
	}

	state.ID = types.StringValue(idhash.HashIDs(append([]string{serviceID}, idsOf(all)...)))
	state.Tags = listVal
	state.Total = types.Int64Value(int64(total))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenTags(items []operations.OperationTag) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The API doesn't promise stable ordering across requests. Sort a copy so
	// the visible list and idhash identifier are deterministic across runs.
	sorted := append([]operations.OperationTag(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	elements := make([]attr.Value, 0, len(sorted))
	for _, t := range sorted {
		obj, objDiags := types.ObjectValue(tagAttrTypes, map[string]attr.Value{
			"created_at":      stringOrNull(t.CreatedAt),
			"description":     stringOrNull(t.Description),
			"id":              types.StringValue(t.ID),
			"name":            types.StringValue(t.Name),
			"operation_count": types.Int64Value(int64(t.Count)),
			"updated_at":      stringOrNull(t.UpdatedAt),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	listValue, listDiags := types.ListValue(types.ObjectType{AttrTypes: tagAttrTypes}, elements)
	diags.Append(listDiags...)

	return listValue, diags
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func idsOf(items []operations.OperationTag) []string {
	ids := make([]string, len(items))
	for i, t := range items {
		ids[i] = t.ID
	}
	return ids
}
