package aclentries

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID      types.String `tfsdk:"id"`
	ACLID   types.String `tfsdk:"acl_id"`
	Entries types.Map    `tfsdk:"entries"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Terraform resource identifier. Format: `acl_id/entries`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"acl_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the ACL that the entries belong to.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"entries": schema.MapAttribute{
			Required:    true,
			ElementType: types.StringType,
			Description: "The ACL entries managed by Terraform, where keys are CIDR prefixes and values are actions (`ALLOW` or `BLOCK`). Entries not declared in this map are left unchanged.",
			Validators: []validator.Map{
				ValidEntries(),
			},
		},
	}
}
