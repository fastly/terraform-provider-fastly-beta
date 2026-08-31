package dnszone

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID               types.String            `tfsdk:"id"`
	Name             types.String            `tfsdk:"name"`
	Description      types.String            `tfsdk:"description"`
	XfrConfigInbound []XfrConfigInboundModel `tfsdk:"xfr_config_inbound"`
}

type XfrConfigInboundModel struct {
	InboundTSIGKeyID types.String   `tfsdk:"inbound_tsig_key_id"`
	Primaries        []PrimaryModel `tfsdk:"primaries"`
}

type PrimaryModel struct {
	Address     types.String `tfsdk:"address"`
	Description types.String `tfsdk:"description"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Zone Identifier.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "The domain name for your zone in FQDN format (e.g. `example.com.`). Must include a trailing period. The API provides no way to rename a zone in place, so changing this attribute will delete and recreate the resource.",
			Validators: []validator.String{
				stringvalidator.RegexMatches(regexp.MustCompile(`\.$`), "must be in FQDN format, ending with a trailing period (e.g. `example.com.`)"),
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"description": schema.StringAttribute{
			Optional:    true,
			Description: "A freeform descriptive note.",
		},
	}
}

// XfrConfigInboundBlock is a singleton block (not a single nested attribute)
// to match the legacy resource's HCL shape; the API only accepts one
// xfr_config_inbound per zone.
func XfrConfigInboundBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "All attributes associated with inbound zone transfers.",
		Validators: []validator.List{
			listvalidator.SizeAtMost(1),
		},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"inbound_tsig_key_id": schema.StringAttribute{
					Optional:    true,
					Description: "The ID of the TSIG key used to secure inbound zone transfers.",
				},
			},
			Blocks: map[string]schema.Block{
				"primaries": schema.ListNestedBlock{
					Description: "An array of the primary DNS server objects associated with inbound zone transfers.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"address": schema.StringAttribute{
								Optional:    true,
								Description: "An IPv4 address for the Primary DNS Server. IPv6 is not supported for DNS zone transfers.",
								Validators: []validator.String{
									ipv4AddressValidator{},
								},
							},
							"description": schema.StringAttribute{
								Optional:    true,
								Description: "A description of the Primary DNS server.",
							},
						},
					},
				},
			},
		},
	}
}
