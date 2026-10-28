package routingconfigauto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v18/fastly/domainmanagement/v1/routingconfigs/paths/rules"
)

func expandRule(rm RuleModel) (rules.Action, []rules.Condition) {
	action := rules.Action{
		Type:  service.StringValue(rm.ActionType),
		Value: service.StringValue(rm.ActionValue),
	}

	conditions := make([]rules.Condition, 0, len(rm.Condition))
	for _, cm := range rm.Condition {
		condition := rules.Condition{
			Operator: service.StringValue(cm.Operator),
			Type:     service.StringValue(cm.Type),
			Value:    service.StringValue(cm.Value),
		}
		if key := service.StringValue(cm.Key); key != "" {
			condition.Key = &key
		}
		conditions = append(conditions, condition)
	}

	return action, conditions
}

// ruleSignature hashes a rule's action and conditions, letting prior and
// desired rules be compared for equality.
func ruleSignature(rm RuleModel) (string, error) {
	action, conditions := expandRule(rm)

	b, err := json.Marshal(struct {
		Action     rules.Action      `json:"action"`
		Conditions []rules.Condition `json:"conditions"`
	}{Action: action, Conditions: conditions})
	if err != nil {
		return "", fmt.Errorf("failed to compute rule signature: %w", err)
	}

	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// sameRules reports whether two rule lists are identical in content and
// order: rules are evaluated top to bottom and the first match wins, and the
// API orders them by creation, so a reorder is a real change.
func sameRules(a, b []RuleModel) (bool, error) {
	if len(a) != len(b) {
		return false, nil
	}

	for i := range a {
		sigA, err := ruleSignature(a[i])
		if err != nil {
			return false, err
		}
		sigB, err := ruleSignature(b[i])
		if err != nil {
			return false, err
		}
		if sigA != sigB {
			return false, nil
		}
	}

	return true, nil
}

// pathsDiffer reports whether two path lists differ, ignoring path order.
func pathsDiffer(a, b []PathModel) (bool, error) {
	if len(a) != len(b) {
		return true, nil
	}

	bByPath := make(map[string]PathModel, len(b))
	for _, pm := range b {
		bByPath[pm.Path.ValueString()] = pm
	}

	for _, pm := range a {
		other, ok := bByPath[pm.Path.ValueString()]
		if !ok {
			return true, nil
		}
		same, err := sameRules(pm.Rule, other.Rule)
		if err != nil {
			return false, err
		}
		if !same {
			return true, nil
		}
	}

	return false, nil
}
