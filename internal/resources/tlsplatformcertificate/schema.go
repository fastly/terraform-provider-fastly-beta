package tlsplatformcertificate

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID                 types.String `tfsdk:"id"`
	CertificateBody    types.String `tfsdk:"certificate_body"`
	IntermediatesBlob  types.String `tfsdk:"intermediates_blob"`
	ConfigurationID    types.String `tfsdk:"configuration_id"`
	AllowUntrustedRoot types.Bool   `tfsdk:"allow_untrusted_root"`
	NotAfter           types.String `tfsdk:"not_after"`
	NotBefore          types.String `tfsdk:"not_before"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
	Replace            types.Bool   `tfsdk:"replace"`
	Domains            types.Set    `tfsdk:"domains"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The unique ID assigned to the certificate by Fastly.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"certificate_body": schema.StringAttribute{
			Required:    true,
			Description: "PEM-formatted certificate.",
			Validators: []validator.String{
				pemBlockValidator{pemType: "CERTIFICATE"},
			},
		},
		"intermediates_blob": schema.StringAttribute{
			Required:    true,
			Description: "PEM-formatted certificate chain from the `certificate_body` to its root.",
			Validators: []validator.String{
				pemBlocksValidator{pemType: "CERTIFICATE"},
			},
		},
		"configuration_id": schema.StringAttribute{
			Required:    true,
			Description: "ID of the TLS configuration to be used to terminate TLS traffic. Changing this attribute will delete and recreate the certificate.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"allow_untrusted_root": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Default:     booldefault.StaticBool(false),
			Description: "Disable checking whether the root of the certificate chain is trusted. Useful for development purposes to allow use of self-signed CAs. Defaults to `false`.",
		},
		"not_after": schema.StringAttribute{
			Computed:    true,
			Description: "Timestamp (GMT) when the certificate will expire. Must be in the future for the certificate to terminate TLS traffic.",
		},
		"not_before": schema.StringAttribute{
			Computed:    true,
			Description: "Timestamp (GMT) when the certificate will become valid. Must be in the past for the certificate to terminate TLS traffic.",
		},
		"created_at": schema.StringAttribute{
			Computed:    true,
			Description: "Timestamp (GMT) when the certificate was created.",
		},
		"updated_at": schema.StringAttribute{
			Computed:    true,
			Description: "Timestamp (GMT) when the certificate was last updated.",
		},
		"replace": schema.BoolAttribute{
			Computed:    true,
			Description: "A recommendation from Fastly indicating the key associated with this certificate is in need of rotation.",
		},
		"domains": schema.SetAttribute{
			ElementType: types.StringType,
			Computed:    true,
			Description: "All the domains (including wildcard domains) that are listed in any certificate's Subject Alternative Names (SAN) list.",
		},
	}
}
