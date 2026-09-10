package apisecuritydiscoveredoperations

import (
	"context"

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
	Status     types.String `tfsdk:"status"`
	Operations types.List   `tfsdk:"operations"`
	Total      types.Int64  `tfsdk:"total"`
}

var operationAttrTypes = map[string]attr.Type{
	"domain":       types.StringType,
	"id":           types.StringType,
	"last_seen_at": types.StringType,
	"method":       types.StringType,
	"path":         types.StringType,
	"rps":          types.Float64Type,
	"status":       types.StringType,
	"updated_at":   types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_security_discovered_operations"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to list API Security discovered operations for a service, optionally filtered by domain, method, path, or status.",
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
				Description: "Filter by one or more fully-qualified domains (exact match).",
			},
			"method": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Filter by one or more HTTP methods.",
			},
			"path": schema.StringAttribute{
				Optional:    true,
				Description: "Filter by path (exact match).",
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Description: "Filter discovered operations by status. Accepted values are `DISCOVERED`, `SAVED`, and `IGNORED`.",
				Validators: []validator.String{
					stringvalidator.OneOf("DISCOVERED", "SAVED", "IGNORED"),
				},
			},
			"operations": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Discovered operations.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"domain": schema.StringAttribute{
							Computed:    true,
							Description: "Discovered operation domain.",
						},
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Discovered operation ID.",
						},
						"last_seen_at": schema.StringAttribute{
							Computed:    true,
							Description: "Last seen timestamp (when present).",
						},
						"method": schema.StringAttribute{
							Computed:    true,
							Description: "Discovered operation HTTP method.",
						},
						"path": schema.StringAttribute{
							Computed:    true,
							Description: "Discovered operation path.",
						},
						"rps": schema.Float64Attribute{
							Computed:    true,
							Description: "Observed requests per second (when present).",
						},
						"status": schema.StringAttribute{
							Computed:    true,
							Description: "Discovered operation status (when present).",
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

	tflog.Debug(ctx, "Reading Fastly API Security discovered operations", map[string]any{"service_id": serviceID})

	in, diags := buildListInput(ctx, serviceID, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	all, err := operations.ListDiscoveredAll(ctx, d.client, in)
	if err != nil {
		resp.Diagnostics.AddError("Error listing API Security discovered operations", err.Error())
		return
	}

	listVal, listDiags := flattenDiscoveredOperations(all)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(idhash.HashIDs(append([]string{serviceID}, idsOf(all)...)))
	state.Operations = listVal
	state.Total = types.Int64Value(int64(len(all)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func buildListInput(ctx context.Context, serviceID string, state DataSourceModel) (*operations.ListDiscoveredInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	page := 0
	limit := pageLimit
	in := &operations.ListDiscoveredInput{
		ServiceID: &serviceID,
		Page:      &page,
		Limit:     &limit,
	}

	if status := service.StringValue(state.Status); status != "" {
		in.Status = &status
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

func flattenDiscoveredOperations(items []operations.DiscoveredOperation) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elements := make([]attr.Value, 0, len(items))
	for _, op := range items {
		obj, objDiags := types.ObjectValue(operationAttrTypes, map[string]attr.Value{
			"domain":       types.StringValue(op.Domain),
			"id":           types.StringValue(op.ID),
			"last_seen_at": stringOrNull(op.LastSeenAt),
			"method":       types.StringValue(op.Method),
			"path":         types.StringValue(op.Path),
			"rps":          types.Float64Value(op.RPS),
			"status":       stringOrNull(op.Status),
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

func idsOf(items []operations.DiscoveredOperation) []string {
	ids := make([]string, len(items))
	for i, op := range items {
		ids[i] = op.ID
	}
	return ids
}
