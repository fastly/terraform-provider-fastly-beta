package tlsprivatekey

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	PrivateKey    types.Object `tfsdk:"private_key"`
	CreatedAt     types.String `tfsdk:"created_at"`
	KeyLength     types.Int64  `tfsdk:"key_length"`
	KeyType       types.String `tfsdk:"key_type"`
	PublicKeySHA1 types.String `tfsdk:"public_key_sha1"`
	Replace       types.Bool   `tfsdk:"replace"`
}

var privateKeyAttributeTypes = map[string]attr.Type{
	"pem": types.StringType,
}

// PEM extracts the pem value from the model's private_key object.
func (m Model) PEM() types.String {
	if m.PrivateKey.IsNull() || m.PrivateKey.IsUnknown() {
		return types.StringNull()
	}
	value, ok := m.PrivateKey.Attributes()["pem"]
	if !ok {
		return types.StringNull()
	}
	pem, ok := value.(types.String)
	if !ok {
		return types.StringNull()
	}
	return pem
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Alphanumeric string identifying a TLS private key.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "A customizable name for your private key.",
			Validators: []validator.String{
				stringvalidator.LengthAtLeast(1),
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"private_key": schema.SingleNestedAttribute{
			Required:    true,
			Description: "The private key material. Updating a private key in place is not supported, so any change here replaces the resource.",
			PlanModifiers: []planmodifier.Object{
				objectplanmodifier.RequiresReplace(),
			},
			Attributes: map[string]schema.Attribute{
				"pem": schema.StringAttribute{
					Required:    true,
					Sensitive:   true,
					Description: "Private key in PEM format. Sensitive key material is not returned once set, so it cannot be read back after creation and will not be populated after a `terraform import`.",
					Validators: []validator.String{
						stringvalidator.LengthAtLeast(1),
					},
				},
			},
		},
		"created_at": schema.StringAttribute{
			Computed:    true,
			Description: "Timestamp (GMT) when the private key was created.",
		},
		"key_length": schema.Int64Attribute{
			Computed:    true,
			Description: "The key length used to generate the private key.",
		},
		"key_type": schema.StringAttribute{
			Computed:    true,
			Description: "The algorithm used to generate the private key. Currently, the only allowed value is `RSA`.",
		},
		"public_key_sha1": schema.StringAttribute{
			Computed:    true,
			Description: "The SHA1 digest of the private key's public key. Useful for safely identifying the key.",
		},
		"replace": schema.BoolAttribute{
			Computed:    true,
			Description: "A recommendation from Fastly to replace this private key and all associated certificates.",
		},
	}
}
