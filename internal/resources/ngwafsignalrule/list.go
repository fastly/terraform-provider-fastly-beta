package ngwafsignalrule

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
		TypeName: "_ngwaf_signal_rule",
		RuleType: RuleType,
		Scope:    scope.ScopeTypeAccount,
		Flatten: func(ctx context.Context, rule *rules.Rule) (any, diag.Diagnostics) {
			model, diags := FlattenToModel(ctx, rule)
			return &model, diags
		},
	})
}
