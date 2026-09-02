// Package ngwafaccountrules implements the fastly_ngwaf_rules data source. It
// is named for the scope rather than for the type name, so it cannot be
// mistaken for the shared ngwafrule package.
package ngwafaccountrules

import (
	"context"
	"sort"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/idhash"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

var _ datasource.DataSource = &DataSource{}

// listLimit is the largest page the rules endpoint accepts.
const listLimit = 1000

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Rules types.List   `tfsdk:"rules"`
}

var ruleAttrTypes = map[string]attr.Type{
	"id":          types.StringType,
	"type":        types.StringType,
	"applies_to":  types.SetType{ElemType: types.StringType},
	"description": types.StringType,
	"enabled":     types.BoolType,
	"created_at":  types.StringType,
	"updated_at":  types.StringType,
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ngwaf_rules"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to retrieve the list of Fastly Next-Gen WAF rules defined at account scope. Each entry reports the workspaces it `applies_to`. For the rules of a single workspace, use the `fastly_ngwaf_workspace_rules` data source instead.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier.",
			},
			"rules": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The list of rules.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The ID of the rule.",
						},
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "The type of the rule, either `request` or `signal` - the two types account scope supports.",
						},
						"applies_to": schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "The workspaces the rule applies to: a set of workspace IDs, or the single entry `*` for every workspace in the account.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "The description of the rule.",
						},
						"enabled": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the rule is currently enabled.",
						},
						"created_at": schema.StringAttribute{
							Computed:    true,
							Description: "The date and time in ISO 8601 format when the rule was created.",
						},
						"updated_at": schema.StringAttribute{
							Computed:    true,
							Description: "The date and time in ISO 8601 format when the rule was last updated.",
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

	d.client = data.Client
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Fastly NGWAF account rules")

	remoteState, err := rules.List(ctx, d.client, &rules.ListInput{
		Limit: new(listLimit),
		Scope: &scope.Scope{Type: scope.ScopeTypeAccount},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error listing NGWAF account rules", err.Error())
		return
	}

	listVal, ids, diags := flattenRules(ctx, remoteState.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Rules = listVal
	state.ID = types.StringValue(idhash.HashIDs(ids))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenRules(ctx context.Context, data []rules.Rule) (types.List, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	sorted := append([]rules.Rule(nil), data...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].RuleID < sorted[j].RuleID
	})

	ids := make([]string, 0, len(sorted))
	elements := make([]attr.Value, 0, len(sorted))

	for _, rule := range sorted {
		ids = append(ids, rule.RuleID)

		appliesTo, appliesToDiags := types.SetValueFrom(ctx, types.StringType, rule.Scope.AppliesTo)
		diags.Append(appliesToDiags...)

		obj, objDiags := types.ObjectValue(ruleAttrTypes, map[string]attr.Value{
			"id":          types.StringValue(rule.RuleID),
			"type":        types.StringValue(rule.Type),
			"applies_to":  appliesTo,
			"description": types.StringValue(rule.Description),
			"enabled":     types.BoolValue(rule.Enabled),
			"created_at":  types.StringValue(rule.CreatedAt.Format("2006-01-02T15:04:05Z")),
			"updated_at":  types.StringValue(rule.UpdatedAt.Format("2006-01-02T15:04:05Z")),
		})
		diags.Append(objDiags...)
		elements = append(elements, obj)
	}

	listValue, listDiags := types.ListValue(types.ObjectType{AttrTypes: ruleAttrTypes}, elements)
	diags.Append(listDiags...)

	return listValue, ids, diags
}
