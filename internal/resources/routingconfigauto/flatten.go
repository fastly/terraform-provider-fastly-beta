package routingconfigauto

import (
	"time"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v18/fastly/domainmanagement/v1/routingconfigs"
	"github.com/fastly/go-fastly/v18/fastly/domainmanagement/v1/routingconfigs/paths/rules"
)

// applyTopLevel copies a routing config's own (non-path) attributes onto m.
func applyTopLevel(m *Model, d *routingconfigs.Data) {
	m.ID = types.StringValue(d.RoutingConfigID)
	m.Name = types.StringValue(d.Name)

	activatedAt := ""
	if d.ActivatedAt != nil {
		activatedAt = d.ActivatedAt.Format(time.RFC3339)
	}
	m.ActivatedAt = types.StringValue(activatedAt)
}

func flattenRule(d rules.Data) RuleModel {
	conditions := make([]ConditionModel, 0, len(d.Conditions))
	for _, c := range d.Conditions {
		conditions = append(conditions, ConditionModel{
			Key:      service.StringPointerOrNull(c.Key),
			Operator: types.StringValue(c.Operator),
			Type:     types.StringValue(c.Type),
			Value:    types.StringValue(c.Value),
		})
	}

	return RuleModel{
		ActionType:  types.StringValue(d.Action.Type),
		ActionValue: types.StringValue(d.Action.Value),
		Condition:   conditions,
		IsDefault:   types.BoolValue(d.IsDefault),
		RuleID:      types.StringValue(d.RuleID),
	}
}
