package servicedictionaryitems

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Terraform resource identifier. Format: `<service_id>/<dictionary_id>`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"service_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the service that the dictionary belongs to.",
			Validators: []validator.String{
				stringvalidator.LengthAtLeast(1),
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"dictionary_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the dictionary that the items belong to.",
			Validators: []validator.String{
				stringvalidator.LengthAtLeast(1),
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"items": schema.MapAttribute{
			Required:    true,
			ElementType: types.StringType,
			Description: "The Dictionary items managed by Terraform, represented as key-value pairs. Items not declared in this map are left unchanged.",
			Validators: []validator.Map{
				ValidItems(),
			},
		},
	}
}
