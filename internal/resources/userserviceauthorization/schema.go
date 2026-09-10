package userserviceauthorization

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// permissions are not exported by go-fastly for this resource, unlike e.g. accesskeys.PERMISSIONS.
var permissions = []string{"full", "read_only", "purge_select", "purge_all"}

// DefaultPermission matches the Fastly API's own server-side default when permission is
// omitted from CreateServiceAuthorization.
const DefaultPermission = "full"

type Model struct {
	ID         types.String `tfsdk:"id"`
	ServiceID  types.String `tfsdk:"service_id"`
	UserID     types.String `tfsdk:"user_id"`
	Permission types.String `tfsdk:"permission"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of this user service authorization.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"service_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the service to grant permissions for.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"user_id": schema.StringAttribute{
			Required:    true,
			Description: "The ID of the user being given access to the service.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"permission": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(DefaultPermission),
			Description: "The permissions to grant the user. Can be `full`, `read_only`, `purge_select` or `purge_all`. Default: `full`.",
			Validators: []validator.String{
				stringvalidator.OneOf(permissions...),
			},
		},
	}
}
