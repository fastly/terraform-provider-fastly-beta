package ngwafsignalrule

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func FlattenToModel(ctx context.Context, rule *rules.Rule) (Model, diag.Diagnostics) {
	appliesTo, diags := ngwafrule.FlattenAppliesTo(ctx, rule)

	return Model{
		CommonModel: ngwafrule.FlattenCommon(rule),
		AppliesTo:   appliesTo,
		Description: types.StringValue(rule.Description),
		Action:      ngwafrule.FlattenSignalActions(rule.Actions),
	}, diags
}
