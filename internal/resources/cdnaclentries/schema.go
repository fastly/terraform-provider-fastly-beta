package cdnaclentries

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID        types.String `tfsdk:"id"`
	ServiceID types.String `tfsdk:"service_id"`
	ACLID     types.String `tfsdk:"acl_id"`
	Entry     types.Set    `tfsdk:"entry"`
}

type EntryModel struct {
	ID      types.String `tfsdk:"id"`
	IP      types.String `tfsdk:"ip"`
	Subnet  types.Int64  `tfsdk:"subnet"`
	Negated types.Bool   `tfsdk:"negated"`
	Comment types.String `tfsdk:"comment"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Alphanumeric string identifying the resource. Format: `service_id/acl_id`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"service_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the Service that the ACL belongs to.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"acl_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the ACL that the items belong to.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
	}
}

func ResourceBlocks() map[string]schema.Block {
	return map[string]schema.Block{
		"entry": schema.SetNestedBlock{
			Description: "ACL entries owned by this resource. Entries not declared here are left unchanged.",
			Validators: []validator.Set{
				setvalidator.SizeAtMost(10000),
				UniqueEntryIdentity(),
			},
			PlanModifiers: []planmodifier.Set{
				preserveEntryIDsModifier{},
			},
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:    true,
						Description: "The unique ID of the entry.",
					},
					"ip": schema.StringAttribute{
						Required:    true,
						Description: "An IP address that is the focus for the ACL.",
					},
					"subnet": schema.Int64Attribute{
						Optional:    true,
						Computed:    true,
						Description: "Number of bits for the subnet mask applied to the IP address (0-32 for IPv4, 0-128 for IPv6).",
						Validators: []validator.Int64{
							int64validator.Between(0, 128),
						},
					},
					"negated": schema.BoolAttribute{
						Optional:    true,
						Computed:    true,
						Description: "A boolean that will negate the match if true.",
					},
					"comment": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: "A personal freeform descriptive note.",
					},
				},
			},
		},
	}
}
