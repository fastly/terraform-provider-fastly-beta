package objectstorageaccesskey

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/objectstorage/accesskeys"
)

type Model struct {
	ID             types.String `tfsdk:"id"`
	AccessKeyID    types.String `tfsdk:"access_key_id"`
	Authentication types.Object `tfsdk:"authentication"`
	Description    types.String `tfsdk:"description"`
	Permission     types.String `tfsdk:"permission"`
	Buckets        types.List   `tfsdk:"buckets"`
}

var authenticationAttributeTypes = map[string]attr.Type{
	"secret_key": types.StringType,
}

func NewAuthenticationObject(secretKey types.String) types.Object {
	return types.ObjectValueMust(authenticationAttributeTypes, map[string]attr.Value{
		"secret_key": secretKey,
	})
}

// SecretKey extracts secret_key from the model's authentication object.
func (m Model) SecretKey() types.String {
	if m.Authentication.IsNull() || m.Authentication.IsUnknown() {
		return types.StringNull()
	}
	value, ok := m.Authentication.Attributes()["secret_key"].(types.String)
	if !ok {
		return types.StringNull()
	}
	return value
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Same value as `access_key_id`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"access_key_id": schema.StringAttribute{
			Computed:    true,
			Description: "ID for the object storage access key.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"authentication": schema.SingleNestedAttribute{
			Computed:    true,
			Description: "Sensitive credential material for the access key.",
			PlanModifiers: []planmodifier.Object{
				objectplanmodifier.UseStateForUnknown(),
			},
			Attributes: map[string]schema.Attribute{
				"secret_key": schema.StringAttribute{
					Computed:    true,
					Sensitive:   true,
					Description: "Secret key for the object storage access key. Only returned at creation time, so it is not populated after a `terraform import`.",
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.UseStateForUnknown(),
					},
				},
			},
		},
		"description": schema.StringAttribute{
			Required:    true,
			Description: "The description of the access key. Access keys cannot be updated, so changing this attribute destroys and recreates the resource.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"permission": schema.StringAttribute{
			Required:    true,
			Description: "The permissions of the access key. Access keys cannot be updated, so changing this attribute destroys and recreates the resource.",
			Validators: []validator.String{
				stringvalidator.OneOf(accesskeys.PERMISSIONS...),
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"buckets": schema.ListAttribute{
			Optional:    true,
			ElementType: types.StringType,
			Description: "The buckets the access key will be associated with. Access keys cannot be updated, so changing this attribute destroys and recreates the resource.",
			PlanModifiers: []planmodifier.List{
				listplanmodifier.RequiresReplace(),
			},
		},
	}
}
