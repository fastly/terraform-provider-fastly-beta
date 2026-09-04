package secretstore

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var nameValidRe = regexp.MustCompile(`^[a-zA-Z0-9\-_.]+$`)

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "An alphanumeric string identifying the Secret Store.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "A human-readable name for the Secret Store. The value must contain only letters, numbers, dashes (-), underscores (_), or periods (.). It is important to note that changing this attribute will delete and recreate the Secret Store, and discard the current entries. You MUST first delete the associated resource_link block from your service before modifying this field.",
			Validators: []validator.String{
				stringvalidator.RegexMatches(nameValidRe, "must contain only letters, numbers, dashes (-), underscores (_), or periods (.)"),
			},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
	}
}
