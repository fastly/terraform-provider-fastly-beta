package routingconfigauto

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	routingConfigActionTypes    = []string{"service"}
	routingConfigConditionTypes = []string{"header"}
	routingConfigOperators      = []string{"equals", "starts_with", "ends_with", "contains"}
	routingConfigPathPattern    = regexp.MustCompile(`^/`)
)

type Model struct {
	ID              types.String `tfsdk:"id"`
	RoutingConfigID types.String `tfsdk:"routing_config_id"`
	Name            types.String `tfsdk:"name"`
	ActivatedAt     types.String `tfsdk:"activated_at"`
	Path            []PathModel  `tfsdk:"path"`
}

type PathModel struct {
	Path   types.String `tfsdk:"path"`
	PathID types.String `tfsdk:"path_id"`
	Rule   []RuleModel  `tfsdk:"rule"`
}

type RuleModel struct {
	ActionType  types.String     `tfsdk:"action_type"`
	ActionValue types.String     `tfsdk:"action_value"`
	Condition   []ConditionModel `tfsdk:"condition"`
	IsDefault   types.Bool       `tfsdk:"is_default"`
	RuleID      types.String     `tfsdk:"rule_id"`
}

type ConditionModel struct {
	Key      types.String `tfsdk:"key"`
	Operator types.String `tfsdk:"operator"`
	Type     types.String `tfsdk:"type"`
	Value    types.String `tfsdk:"value"`
}

func ResourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of this resource (identical to `routing_config_id`).",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"routing_config_id": schema.StringAttribute{
			Computed:    true,
			Description: "The routing config identifier.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "The user-defined name of the routing config. Can be created, but not updated.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"activated_at": schema.StringAttribute{
			Computed:    true,
			Description: "The date and time the routing config's active version was last activated, in ISO 8601 format.",
		},
	}
}

// PathBlock requires at least one entry (enforced in ValidateConfig): a
// routing config with no paths has nothing to reconcile or activate.
func PathBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "A URL path pattern and the rules used to route matching requests. At least one `path` block is required.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"path": schema.StringAttribute{
					Required:    true,
					Description: "The URL path pattern (max 2048 characters, starts with `/`).",
					Validators: []validator.String{
						stringvalidator.LengthAtMost(2048),
						stringvalidator.RegexMatches(routingConfigPathPattern, "must start with `/`"),
					},
				},
				"path_id": schema.StringAttribute{
					Computed:    true,
					Description: "The path identifier.",
				},
			},
			Blocks: map[string]schema.Block{
				"rule": RuleBlock(),
			},
		},
	}
}

func RuleBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "A conditional routing rule for this path; each `path` requires at least one, including a default rule. Rules are evaluated in the order listed and the first match wins, so order matters. A rule with no `condition` blocks is the default (catch-all) rule for the path.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"action_type": schema.StringAttribute{
					Required:    true,
					Description: "The action type. Currently only `service` is supported.",
					Validators: []validator.String{
						stringvalidator.OneOf(routingConfigActionTypes...),
					},
				},
				"action_value": schema.StringAttribute{
					Required:    true,
					Description: "The destination for the action (e.g. a service ID when `action_type` is `service`).",
				},
				"is_default": schema.BoolAttribute{
					Computed:    true,
					Description: "Whether this is the catch-all rule for its path (i.e. it has no conditions).",
				},
				"rule_id": schema.StringAttribute{
					Computed:    true,
					Description: "The rule identifier.",
				},
			},
			Blocks: map[string]schema.Block{
				"condition": ConditionBlock(),
			},
		},
	}
}

func ConditionBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "A matching criterion evaluated against the incoming request. Omit to make this the default (catch-all) rule for the path.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"key": schema.StringAttribute{
					Optional:    true,
					Description: "The header name to match against. Required when `type` is `header`.",
				},
				"operator": schema.StringAttribute{
					Required:    true,
					Description: "The comparison operator used to evaluate `value` against the request. One of `equals`, `starts_with`, `ends_with`, or `contains`.",
					Validators: []validator.String{
						stringvalidator.OneOf(routingConfigOperators...),
					},
				},
				"type": schema.StringAttribute{
					Required:    true,
					Description: "The condition category. Currently only `header` is supported.",
					Validators: []validator.String{
						stringvalidator.OneOf(routingConfigConditionTypes...),
					},
				},
				"value": schema.StringAttribute{
					Required:    true,
					Description: "The value compared against the request using `operator`.",
				},
			},
		},
	}
}
