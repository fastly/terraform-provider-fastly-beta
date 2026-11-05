package ngwafworkspacetemplatedsignalrule

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"

	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/scope"
)

func NewListResource() list.ListResource {
	return ngwafrule.NewListResource(ngwafrule.ListDefinition{
		TypeName: "_ngwaf_workspace_templated_signal_rule",
		RuleType: RuleType,
		Scope:    scope.ScopeTypeWorkspace,
		Flatten: func(_ context.Context, rule *rules.Rule) (any, diag.Diagnostics) {
			model := FlattenToModel(rule)
			return &model, nil
		},
	})
}
