package apisecurityoperations

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
	ID         types.String `tfsdk:"id"`
	ServiceID  types.String `tfsdk:"service_id"`
	Domain     types.Set    `tfsdk:"domain"`
	Method     types.Set    `tfsdk:"method"`
	Path       types.String `tfsdk:"path"`
	TagID      types.String `tfsdk:"tag_id"`
	Operations types.List   `tfsdk:"operations"`
	Total      types.Int64  `tfsdk:"total"`
}

var operationAttrTypes = map[string]attr.Type{
	"created_at":   types.StringType,
	"description":  types.StringType,
	"domain":       types.StringType,
	"id":           types.StringType,
	"last_seen_at": types.StringType,
	"method":       types.StringType,
	"path":         types.StringType,
	"rps":          types.Float64Type,
	"status":       types.StringType,
	"tag_ids":      types.SetType{ElemType: types.StringType},
	"updated_at":   types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_security_operations"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to list API Security operations for a service, optionally filtered by domain, method, path, or tag.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"service_id": schema.StringAttribute{
				Required:    true,
				Description: "Service ID.",
			},
			"domain": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Filter by one or more domains (exact match).",
			},
			"method": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Filter by one or more HTTP methods.",
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(
						stringvalidator.OneOf(
							"GET",
							"POST",
							"PUT",
							"PATCH",
							"DELETE",
							"HEAD",
							"OPTIONS",
							"CONNECT",
							"TRACE",
						),
					),
				},
			},
			"path": schema.StringAttribute{
				Optional:    true,
				Description: "Filter by path (exact match).",
			},
			"tag_id": schema.StringAttribute{
				Optional:    true,
				Description: "Filter by tag ID.",
			},
			"operations": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Matching API Security operations.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "Created timestamp (when present).",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "Operation description (when present).",
						},
						"domain": schema.StringAttribute{
							Computed:    true,
							Description: "Operation domain.",
						},
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Operation ID.",
						},
						"last_seen_at": schema.StringAttribute{
							Computed:    true,
							Description: "Last seen timestamp (when present).",
						},
						"method": schema.StringAttribute{
							Computed:    true,
							Description: "Operation HTTP method.",
						},
						"path": schema.StringAttribute{
							Computed:    true,
							Description: "Operation path.",
						},
						"rps": schema.Float64Attribute{
							Computed:    true,
							Description: "Observed requests per second (when present).",
						},
						"status": schema.StringAttribute{
							Computed:    true,
							Description: "Discovery status (when present). One of `DISCOVERED`, `SAVED`, or `IGNORED`.",
						},
						"tag_ids": schema.SetAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "Associated operation tag IDs.",
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

	tflog.Debug(ctx, "Reading Fastly API Security operations", map[string]any{"service_id": serviceID})

	in, diags := buildListInput(ctx, serviceID, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The first page carries meta.total, which the API may compute from a
	// different (e.g. cached or eventually-consistent) count than the number of
	// items ListOperationsAll ends up actually returning; fall back to that
	// count only if the API doesn't report one.
	first, err := operations.ListOperations(ctx, d.client, in)
	if err != nil {
		resp.Diagnostics.AddError("Error listing API Security operations", err.Error())
		return
	}

	all, err := operations.ListOperationsAll(ctx, d.client, in)
	if err != nil {
		resp.Diagnostics.AddError("Error listing API Security operations", err.Error())
		return
	}

	listVal, listDiags := flattenOperations(ctx, all)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	total := first.Meta.Total
	if total == 0 {
		total = len(all)
	}

	state.ID = types.StringValue(idhash.HashIDs(append([]string{serviceID}, idsOf(all)...)))
	state.Operations = listVal
	state.Total = types.Int64Value(int64(total))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func buildListInput(ctx context.Context, serviceID string, state DataSourceModel) (*operations.ListOperationsInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	page := 0
	limit := pageLimit
	in := &operations.ListOperationsInput{
		ServiceID: &serviceID,
		Page:      &page,
		Limit:     &limit,
	}

	if tagID := service.StringValue(state.TagID); tagID != "" {
		in.TagID = &tagID
	}
	if path := service.StringValue(state.Path); path != "" {
		in.Path = &path
	}

	if !state.Method.IsNull() && !state.Method.IsUnknown() {
		var methods []string
		diags.Append(state.Method.ElementsAs(ctx, &methods, false)...)
		in.Method = methods
	}
	if !state.Domain.IsNull() && !state.Domain.IsUnknown() {
		var domains []string
		diags.Append(state.Domain.ElementsAs(ctx, &domains, false)...)
		in.Domain = domains
	}

	return in, diags
}

func flattenOperations(ctx context.Context, items []operations.Operation) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The API doesn't promise stable ordering across requests. Sort a copy so
	// the visible list and idhash identifier are deterministic across runs.
	sorted := append([]operations.Operation(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	elements := make([]attr.Value, 0, len(sorted))
	for _, op := range sorted {
		tagIDs, tagIDsDiags := types.SetValueFrom(ctx, types.StringType, op.TagIDs)
		diags.Append(tagIDsDiags...)

		obj, objDiags := types.ObjectValue(operationAttrTypes, map[string]attr.Value{
			"created_at":   stringOrNull(op.CreatedAt),
			"description":  stringOrNull(op.Description),
			"domain":       types.StringValue(op.Domain),
			"id":           types.StringValue(op.ID),
			"last_seen_at": stringOrNull(op.LastSeenAt),
			"method":       types.StringValue(op.Method),
			"path":         types.StringValue(op.Path),
			"rps":          types.Float64Value(op.RPS),
			"status":       stringOrNull(op.Status),
			"tag_ids":      tagIDs,
			"updated_at":   stringOrNull(op.UpdatedAt),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	listValue, listDiags := types.ListValue(types.ObjectType{AttrTypes: operationAttrTypes}, elements)
	diags.Append(listDiags...)

	return listValue, diags
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func idsOf(items []operations.Operation) []string {
	ids := make([]string, len(items))
	for i, op := range items {
		ids[i] = op.ID
	}
	return ids
}
