package dictionaries

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
	"github.com/fastly/terraform-provider-fastly-beta/internal/validation"

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
	providerData *fastlyclient.Data
}

type DataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	ServiceID      types.String `tfsdk:"service_id"`
	ServiceVersion types.Int64  `tfsdk:"service_version"`
	Dictionaries   types.Set    `tfsdk:"dictionaries"`
}

var dictionaryAttrTypes = map[string]attr.Type{
	"id":         types.StringType,
	"name":       types.StringType,
	"write_only": types.BoolType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dictionaries"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve a list of Fastly dictionaries for a service version.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"service_id": schema.StringAttribute{
				Required:    true,
				Description: "Fastly service ID.",
			},
			"service_version": schema.Int64Attribute{
				Required:    true,
				Description: "Fastly service version to read dictionaries from.",
			},
			"dictionaries": schema.SetNestedAttribute{
				Computed:    true,
				Description: "List of all dictionaries for the configured service version.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Alphanumeric string identifying the dictionary.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Name of the dictionary.",
						},
						"write_only": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether items in the dictionary are readable or not.",
						},
					},
				},
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

	d.providerData = data
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ServiceID.ValueString()
	version := int(state.ServiceVersion.ValueInt64())

	if err := validation.EnsureServiceTypeSupported(ctx, d.providerData.TypeChecker, serviceID, "fastly_dictionaries", service.TypeVCL, service.TypeCompute); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	tflog.Debug(ctx, "Reading Fastly dictionaries", map[string]any{
		"service_id":      serviceID,
		"service_version": version,
	})

	dictionaries, err := d.providerData.Client.ListDictionaries(ctx, &fastly.ListDictionariesInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading dictionaries", err.Error())
		return
	}

	setVal, ids, diags := flattenDictionaries(dictionaries)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Dictionaries = setVal
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenDictionaries(dictionaries []*fastly.Dictionary) (types.Set, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids := make([]string, 0, len(dictionaries))
	elements := make([]attr.Value, 0, len(dictionaries))

	for _, dict := range dictionaries {
		if dict == nil {
			continue
		}

		id := fastly.ToValue(dict.DictionaryID)
		ids = append(ids, id)

		obj, objDiags := types.ObjectValue(dictionaryAttrTypes, map[string]attr.Value{
			"id":         types.StringValue(id),
			"name":       types.StringValue(fastly.ToValue(dict.Name)),
			"write_only": types.BoolValue(fastly.ToValue(dict.WriteOnly)),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: dictionaryAttrTypes}), nil, diags
	}

	setVal, setDiags := types.SetValue(types.ObjectType{AttrTypes: dictionaryAttrTypes}, elements)
	diags.Append(setDiags...)

	return setVal, ids, diags
}
