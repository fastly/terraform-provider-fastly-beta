// Package ngwafrequestrule implements the fastly_ngwaf_request_rule resource.
package ngwafrequestrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// RuleType is the API's rule `type` discriminator for this resource.
const RuleType = "request"

// DefaultRequestLogging is the value the API assigns to request_logging when
// it's omitted from the request body.
const DefaultRequestLogging = "sampled"

var (
	// actionTypes is confirmed against the live API, not the spec: the spec's
	// AccountRequestRuleBody lists the same seven action types as the workspace
	// body, but the account endpoint rejects the challenge and deception
	// families. Kept separate from the workspace enum for that reason.
	actionTypes          = []string{"allow", "block", "add_signal"}
	requestLoggingValues = []string{"sampled", "none"}
)

// Model is the fastly_ngwaf_request_rule resource state.
type Model struct {
	ngwafrule.CommonModel
	AppliesTo      types.Set                      `tfsdk:"applies_to"`
	Description    types.String                   `tfsdk:"description"`
	RequestLogging types.String                   `tfsdk:"request_logging"`
	Action         []ngwafrule.AccountActionModel `tfsdk:"action"`
}

func resourceAttributes() map[string]schema.Attribute {
	attributes := ngwafrule.CommonAttributes()

	attributes["applies_to"] = ngwafrule.AppliesToAttribute()
	attributes["description"] = ngwafrule.DescriptionAttribute()
	attributes["request_logging"] = schema.StringAttribute{
		Optional:    true,
		Computed:    true,
		Default:     stringdefault.StaticString(DefaultRequestLogging),
		Description: "Logging behavior for matching requests. " + ngwafrule.OneOfDescriptor(requestLoggingValues) + " Defaults to `" + DefaultRequestLogging + "`.",
		Validators: []validator.String{
			stringvalidator.OneOf(requestLoggingValues...),
		},
	}

	return attributes
}

func resourceBlocks() map[string]schema.Block {
	blocks := ngwafrule.ConditionBlocks()
	blocks["action"] = ngwafrule.AccountActionBlock(actionTypes, 1, 2)
	return blocks
}
