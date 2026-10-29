package auditlogeventmapping

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	resourcepkg "github.com/fastly/terraform-provider-fastly-beta/internal/resources/auditlogeventmapping"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v18/fastly"
	"github.com/fastly/go-fastly/v18/fastly/notifications/v1/eventmappings"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	resourcepkg.Model
	IntegrationID types.String `tfsdk:"integration_id"`
	ScopeID       types.String `tfsdk:"scope_id"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_audit_log_event_mapping"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	conflictsWithID := []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("id"))}

	resp.Schema = schema.Schema{
		Description: "Use this data source to get the details of an Audit Log Event Mapping, by ID or by other lookup criteria.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Unique ID of the mapping. Conflicts with all the other filters.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("integration_id"),
						path.MatchRoot("mapping_status"),
						path.MatchRoot("name"),
						path.MatchRoot("scope_id"),
						path.MatchRoot("scope_type"),
					),
				},
			},
			"integration_id": schema.StringAttribute{
				Optional:    true,
				Description: "Filters results to mappings that reference the given integration ID.",
				Validators:  conflictsWithID,
			},
			"mapping_status": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Filters results by mapping status: `active` or `inactive`.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
					stringvalidator.OneOf(eventmappings.MappingStatusActive, eventmappings.MappingStatusInactive),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Filters results to mappings whose name contains the given string (case-insensitive).",
				Validators:  conflictsWithID,
			},
			"scope_id": schema.StringAttribute{
				Optional:    true,
				Description: "Filters results to mappings that apply to the given service or workspace ID.",
				Validators:  conflictsWithID,
			},
			"scope_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: fmt.Sprintf("Filters results to the given scope type: %s.", resourcepkg.ScopeTypesList()),
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
					stringvalidator.OneOf(resourcepkg.ScopeTypes...),
				},
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the mapping.",
			},
			"scope_ids": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The specific service or workspace IDs the mapping is scoped to.",
			},
			"event_types": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The audit event types that trigger a notification.",
			},
			"integration_ids": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The IDs of the integrations that receive notifications.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (RFC3339) when the mapping was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (RFC3339) when the mapping was last updated.",
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
	var config DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var mapping *eventmappings.EventMapping

	if id := service.StringValue(config.ID); id != "" {
		tflog.Debug(ctx, "Reading Fastly Audit Log Event Mapping", map[string]any{"id": id})

		m, err := eventmappings.Get(ctx, d.client, &eventmappings.GetInput{MappingID: new(id)})
		if err != nil {
			resp.Diagnostics.AddError("Error reading Audit Log Event Mapping", err.Error())
			return
		}
		mapping = m
	} else {
		input := &eventmappings.ListInput{}
		if v := service.StringValue(config.Name); v != "" {
			input.Name = new(v)
		}
		if v := service.StringValue(config.ScopeType); v != "" {
			input.ScopeType = new(v)
		}
		if v := service.StringValue(config.ScopeID); v != "" {
			input.ScopeID = new(v)
		}
		if v := service.StringValue(config.IntegrationID); v != "" {
			input.IntegrationID = new(v)
		}
		if v := service.StringValue(config.MappingStatus); v != "" {
			input.MappingStatus = new(v)
		}

		tflog.Debug(ctx, "Listing Fastly Audit Log Event Mappings")

		mappings, err := eventmappings.List(ctx, d.client, input)
		if err != nil {
			resp.Diagnostics.AddError("Error listing Audit Log Event Mappings", err.Error())
			return
		}

		switch len(mappings) {
		case 0:
			resp.Diagnostics.AddError("No results", "Your query returned no results. Please change your search criteria and try again.")
			return
		case 1:
			mapping = &mappings[0]
		default:
			resp.Diagnostics.AddError("Multiple results", "Your query returned more than one result. Please try a more specific search criteria and try again.")
			return
		}
	}

	model, diags := resourcepkg.FlattenToModel(ctx, mapping, resourcepkg.Model{
		Description: types.StringNull(),
		ScopeIDs:    types.SetNull(types.StringType),
	})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := DataSourceModel{
		Model:         model,
		IntegrationID: config.IntegrationID,
		ScopeID:       config.ScopeID,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
