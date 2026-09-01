// Package ngwafworkspaceratelimitrule implements the
// fastly_ngwaf_workspace_rate_limit_rule resource.
package ngwafworkspaceratelimitrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// RuleType is the API's rule `type` discriminator for this resource.
const RuleType = "rate_limit"

var (
	actionTypes           = []string{"block_signal", "log_request", "browser_challenge", "dynamic_challenge", "deception"}
	rateLimitIntervals    = []int64{60, 600, 3600}
	rateLimitDurationMin  = int64(300)
	rateLimitDurationMax  = int64(86400)
	rateLimitThresholdMin = int64(1)
	rateLimitThresholdMax = int64(100000)
	clientIdentifierTypes = []string{"ip", "post_parameter", "request_cookie", "request_header", "signal_payload"}
)

// Model is the fastly_ngwaf_workspace_rate_limit_rule resource state.
type Model struct {
	ngwafrule.CommonModel
	WorkspaceID types.String            `tfsdk:"workspace_id"`
	Description types.String            `tfsdk:"description"`
	Action      []ngwafrule.ActionModel `tfsdk:"action"`
	RateLimit   []RateLimitModel        `tfsdk:"rate_limit"`
}

// RateLimitModel configures the rule's threshold and window.
type RateLimitModel struct {
	ClientIdentifiers []ClientIdentifierModel `tfsdk:"client_identifiers"`
	Duration          types.Int64             `tfsdk:"duration"`
	Interval          types.Int64             `tfsdk:"interval"`
	Signal            types.String            `tfsdk:"signal"`
	Threshold         types.Int64             `tfsdk:"threshold"`
}

// ClientIdentifierModel names one attribute of the request used to group
// requests for rate limiting.
type ClientIdentifierModel struct {
	Key    types.String `tfsdk:"key"`
	Name   types.String `tfsdk:"name"`
	Signal types.String `tfsdk:"signal"`
	Type   types.String `tfsdk:"type"`
}

func resourceAttributes() map[string]schema.Attribute {
	attributes := ngwafrule.CommonAttributes()
	attributes["workspace_id"] = ngwafrule.WorkspaceIDAttribute()
	attributes["description"] = ngwafrule.DescriptionAttribute()
	return attributes
}

func resourceBlocks() map[string]schema.Block {
	blocks := ngwafrule.ConditionBlocks()
	blocks["action"] = ngwafrule.ActionBlock(actionTypes, 1, 1)
	blocks["rate_limit"] = rateLimitBlock()
	return blocks
}

func rateLimitBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "The rate limit this rule enforces. Required, and must contain exactly 1 entry.",
		Validators: []validator.List{
			listvalidator.IsRequired(),
			listvalidator.SizeBetween(1, 1),
		},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"duration": schema.Int64Attribute{
					Required:    true,
					Description: "Duration in seconds for the rate limit. " + ngwafrule.RangeDescriptor(rateLimitDurationMin, rateLimitDurationMax),
					Validators: []validator.Int64{
						int64validator.Between(rateLimitDurationMin, rateLimitDurationMax),
					},
				},
				"interval": schema.Int64Attribute{
					Required:    true,
					Description: "Time interval for the rate limit in seconds. " + ngwafrule.OneOfDescriptor(rateLimitIntervals),
					Validators: []validator.Int64{
						int64validator.OneOf(rateLimitIntervals...),
					},
				},
				"signal": schema.StringAttribute{
					Required:    true,
					Description: "Reference ID of the custom signal this rule uses to count requests.",
					Validators: []validator.String{
						stringvalidator.LengthAtLeast(1),
					},
				},
				"threshold": schema.Int64Attribute{
					Required:    true,
					Description: "Rate limit threshold. " + ngwafrule.RangeDescriptor(rateLimitThresholdMin, rateLimitThresholdMax),
					Validators: []validator.Int64{
						int64validator.Between(rateLimitThresholdMin, rateLimitThresholdMax),
					},
				},
			},
			Blocks: map[string]schema.Block{
				"client_identifiers": clientIdentifiersBlock(),
			},
		},
	}
}

func clientIdentifiersBlock() schema.SetNestedBlock {
	return schema.SetNestedBlock{
		Description: "List of client identifiers used for rate limiting. Required, and must contain 1 or 2 entries.",
		Validators: []validator.Set{
			setvalidator.IsRequired(),
			setvalidator.SizeBetween(1, 2),
		},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"key": schema.StringAttribute{
					Optional:    true,
					Description: "Key for the Client Identifier. Only valid when `type` is `request_header`; excluded otherwise.",
				},
				"name": schema.StringAttribute{
					Optional:    true,
					Description: "Name for the Client Identifier. Required when `type` is `request_header`, `request_cookie`, or `post_parameter`; excluded when `type` is `ip` or `signal_payload`.",
				},
				"signal": schema.StringAttribute{
					Optional:    true,
					Description: "Signal for the Client Identifier. Required when `type` is `signal_payload`; excluded otherwise.",
				},
				"type": schema.StringAttribute{
					Required:    true,
					Description: "Type of the Client Identifier. " + ngwafrule.OneOfDescriptor(clientIdentifierTypes),
					Validators: []validator.String{
						stringvalidator.OneOf(clientIdentifierTypes...),
					},
				},
			},
		},
	}
}
