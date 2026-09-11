package services

import (
	"context"
	"time"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Details types.Set    `tfsdk:"details"`
	IDs     types.Set    `tfsdk:"ids"`
}

var detailAttrTypes = map[string]attr.Type{
	"comment":     types.StringType,
	"created_at":  types.StringType,
	"customer_id": types.StringType,
	"id":          types.StringType,
	"name":        types.StringType,
	"type":        types.StringType,
	"updated_at":  types.StringType,
	"version":     types.Int64Type,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_services"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly services in your account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"details": schema.SetNestedAttribute{
				Computed:    true,
				Description: "A detailed list of Fastly services in your account. This is limited to the services the API token can read.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"comment": schema.StringAttribute{
							Computed:    true,
							Description: "A freeform descriptive note.",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "Date and time in ISO 8601 format.",
						},
						"customer_id": schema.StringAttribute{
							Computed:    true,
							Description: "Alphanumeric string identifying the customer.",
						},
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Alphanumeric string identifying the service.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the service.",
						},
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "The type of this service. One of `vcl`, `wasm`.",
						},
						"updated_at": schema.StringAttribute{
							Computed:    true,
							Description: "Date and time in ISO 8601 format.",
						},
						"version": schema.Int64Attribute{
							Computed:    true,
							Description: "The currently activated version.",
						},
					},
				},
			},
			"ids": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "A list of service IDs in your account. This is limited to the services the API token can read.",
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

	tflog.Debug(ctx, "Reading Fastly services")

	remoteState, err := d.client.ListServices(ctx, &fastly.ListServicesInput{})
	if err != nil {
		resp.Diagnostics.AddError("Error listing services", err.Error())
		return
	}

	detailsSet, ids, diags := flattenServices(remoteState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	idsSet, diags := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Details = detailsSet
	state.IDs = idsSet
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenServices(services []*fastly.Service) (types.Set, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids := make([]string, 0, len(services))
	elements := make([]attr.Value, 0, len(services))

	for _, svc := range services {
		if svc == nil {
			continue
		}

		id := fastly.ToValue(svc.ServiceID)
		ids = append(ids, id)

		obj, objDiags := types.ObjectValue(detailAttrTypes, map[string]attr.Value{
			"comment":     service.StringPointerOrNull(svc.Comment),
			"created_at":  timeOrNull(svc.CreatedAt),
			"customer_id": service.StringPointerOrNull(svc.CustomerID),
			"id":          types.StringValue(id),
			"name":        service.StringPointerOrNull(svc.Name),
			"type":        service.StringPointerOrNull(svc.Type),
			"updated_at":  timeOrNull(svc.UpdatedAt),
			"version":     service.Int64PointerOrNull(svc.ActiveVersion),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	setValue, setDiags := types.SetValue(types.ObjectType{AttrTypes: detailAttrTypes}, elements)
	diags.Append(setDiags...)

	return setValue, ids, diags
}

func timeOrNull(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
