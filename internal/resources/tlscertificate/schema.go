package tlscertificate

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID                 types.String `tfsdk:"id"`
	CertificateBody    types.String `tfsdk:"certificate_body"`
	CreatedAt          types.String `tfsdk:"created_at"`
	Domains            types.Set    `tfsdk:"domains"`
	IssuedTo           types.String `tfsdk:"issued_to"`
	Issuer             types.String `tfsdk:"issuer"`
	Name               types.String `tfsdk:"name"`
	Replace            types.Bool   `tfsdk:"replace"`
	SerialNumber       types.String `tfsdk:"serial_number"`
	SignatureAlgorithm types.String `tfsdk:"signature_algorithm"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Alphanumeric string identifying a TLS certificate.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"certificate_body": schema.StringAttribute{
			Required:    true,
			Description: "PEM-formatted certificate, optionally including any intermediary certificates.",
			Validators:  []validator.String{pemBlocks{}},
		},
		"created_at": schema.StringAttribute{
			Computed:    true,
			Description: "Timestamp (GMT) when the certificate was created.",
		},
		"domains": schema.SetAttribute{
			Computed:    true,
			ElementType: types.StringType,
			Description: "All the domains (including wildcard domains) that are listed in the certificate's Subject Alternative Names (SAN) list.",
		},
		"issued_to": schema.StringAttribute{
			Computed:    true,
			Description: "The hostname for which a certificate was issued.",
		},
		"issuer": schema.StringAttribute{
			Computed:    true,
			Description: "The certificate authority that issued the certificate.",
		},
		"name": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Description: "Human-readable name used to identify the certificate. Defaults to the certificate's Common Name or first Subject Alternative Name entry.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"replace": schema.BoolAttribute{
			Computed:    true,
			Description: "A recommendation from Fastly indicating the key associated with this certificate is in need of rotation.",
		},
		"serial_number": schema.StringAttribute{
			Computed:    true,
			Description: "A value assigned by the issuer that is unique to a certificate.",
		},
		"signature_algorithm": schema.StringAttribute{
			Computed:    true,
			Description: "The algorithm used to sign the certificate.",
		},
		"updated_at": schema.StringAttribute{
			Computed:    true,
			Description: "Timestamp (GMT) when the certificate was last updated.",
		},
	}
}
