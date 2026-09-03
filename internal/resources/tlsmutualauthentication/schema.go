package tlsmutualauthentication

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID             types.String `tfsdk:"id"`
	ActivationIDs  types.Set    `tfsdk:"activation_ids"`
	CertBundle     types.String `tfsdk:"cert_bundle"`
	CreatedAt      types.String `tfsdk:"created_at"`
	Enforced       types.Bool   `tfsdk:"enforced"`
	Include        types.String `tfsdk:"include"`
	Name           types.String `tfsdk:"name"`
	TLSActivations types.List   `tfsdk:"tls_activations"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Alphanumeric string identifying a mutual authentication.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"activation_ids": schema.SetAttribute{
			Optional:    true,
			ElementType: types.StringType,
			Description: "List of TLS Activation IDs",
		},
		"cert_bundle": schema.StringAttribute{
			Required:    true,
			Description: "One or more certificates. Enter each individual certificate blob on a new line. Must be PEM-formatted.",
		},
		"created_at": schema.StringAttribute{
			Computed:    true,
			Description: "Date and time in ISO 8601 format.",
		},
		"enforced": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Description: "Determines whether Mutual TLS will fail closed (enforced) or fail open. A true value will require a successful Mutual TLS handshake for the connection to continue and will fail closed if unsuccessful. A false value will fail open and allow the connection to proceed (if this attribute is not set we default to `false`).",
			PlanModifiers: []planmodifier.Bool{
				boolplanmodifier.UseStateForUnknown(),
			},
		},
		"include": schema.StringAttribute{
			Optional:    true,
			Description: "A comma-separated list used by the Terraform provider during a state refresh to return more data related to your mutual authentication from the Fastly API (permitted values: `tls_activations`).",
		},
		"name": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Description: "A custom name for your mutual authentication. If name is not supplied we will auto-generate one.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"tls_activations": schema.ListAttribute{
			Computed:    true,
			ElementType: types.StringType,
			Description: "List of alphanumeric strings identifying TLS activations.",
		},
		"updated_at": schema.StringAttribute{
			Computed:    true,
			Description: "Date and time in ISO 8601 format.",
		},
	}
}
