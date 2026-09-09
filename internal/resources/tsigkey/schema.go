package tsigkey

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID          types.String `tfsdk:"id"`
	Algorithm   types.String `tfsdk:"algorithm"`
	Description types.String `tfsdk:"description"`
	Name        types.String `tfsdk:"name"`
	Secret      types.Object `tfsdk:"secret"`
}

var secretAttributeTypes = map[string]attr.Type{
	"value": types.StringType,
}

func (m Model) SecretValue() types.String {
	if m.Secret.IsNull() || m.Secret.IsUnknown() {
		return types.StringNull()
	}
	v, ok := m.Secret.Attributes()["value"]
	if !ok {
		return types.StringNull()
	}
	s, ok := v.(types.String)
	if !ok {
		return types.StringNull()
	}
	return s
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "TSIG Key Identifier (UUID).",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"algorithm": schema.StringAttribute{
			Required:    true,
			Description: "The algorithm of the TSIG key. One of: `hmac-sha224`, `hmac-sha256`, `hmac-sha384`, `hmac-sha512`.",
			Validators: []validator.String{
				stringvalidator.OneOf("hmac-sha224", "hmac-sha256", "hmac-sha384", "hmac-sha512"),
			},
		},
		"description": schema.StringAttribute{
			Optional:    true,
			Description: "A freeform descriptive note.",
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "The name of the TSIG key.",
			Validators: []validator.String{
				stringvalidator.LengthBetween(1, 255),
				stringvalidator.RegexMatches(regexp.MustCompile(`^\S+$`), "must not contain spaces"),
			},
		},
		"secret": schema.SingleNestedAttribute{
			Required:    true,
			Description: "The TSIG key's shared secret.",
			Attributes: map[string]schema.Attribute{
				"value": schema.StringAttribute{
					Required:    true,
					Sensitive:   true,
					Description: "The Base64 encoded secret key. Sensitive key material is not returned once set, so it cannot be read back after creation and will not be populated after a `terraform import`.",
					Validators: []validator.String{
						base64Validator{},
					},
				},
			},
		},
	}
}
