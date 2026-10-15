package routingconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/routingconfigs/paths/rules"
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

// ruleSignature hashes a rule's action and conditions (sorted, so order is
// ignored), letting prior and desired rules be compared for equality.
func ruleSignature(rm RuleModel) (string, error) {
	action, conditions := expandRule(rm)

	sort.Slice(conditions, func(i, j int) bool {
		ki, kj := "", ""
		if conditions[i].Key != nil {
			ki = *conditions[i].Key
		}
		if conditions[j].Key != nil {
			kj = *conditions[j].Key
		}
		if conditions[i].Type != conditions[j].Type {
			return conditions[i].Type < conditions[j].Type
		}
		if ki != kj {
			return ki < kj
		}
		if conditions[i].Operator != conditions[j].Operator {
			return conditions[i].Operator < conditions[j].Operator
		}
		return conditions[i].Value < conditions[j].Value
	})

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

// sameRuleSet reports whether two rule sets are equivalent, ignoring order.
func sameRuleSet(a, b []RuleModel) (bool, error) {
	if len(a) != len(b) {
		return false, nil
	}

	sigsA := make([]string, len(a))
	for i, rm := range a {
		sig, err := ruleSignature(rm)
		if err != nil {
			return false, err
		}
		sigsA[i] = sig
	}

	sigsB := make([]string, len(b))
	for i, rm := range b {
		sig, err := ruleSignature(rm)
		if err != nil {
			return false, err
		}
		sigsB[i] = sig
	}

	sort.Strings(sigsA)
	sort.Strings(sigsB)

	for i := range sigsA {
		if sigsA[i] != sigsB[i] {
			return false, nil
		}
	}

	return true, nil
}
