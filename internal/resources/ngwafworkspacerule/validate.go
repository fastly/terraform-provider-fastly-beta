package ngwafworkspacerule

import (
	"fmt"

	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"
)

// MaxConditions caps the combined count of top-level
// condition/group_condition/multival_condition entries. A group_condition
// or multival_condition counts as one entry regardless of how many
// conditions it nests inside.
const MaxConditions = 10

// TotalConditionCount returns the combined number of top-level condition,
// group_condition, and multival_condition entries a rule declares.
func TotalConditionCount(conditions []ngwafrule.ConditionModel, groups []ngwafrule.GroupConditionModel, multivals []ngwafrule.MultivalConditionModel) int {
	return len(conditions) + len(groups) + len(multivals)
}

// InvalidDescription returns a non-empty message if description doesn't
// match its rule type's requirement: templated_signal rules must be
// empty, every other type non-empty. The schema marks it Required even
// for templated_signal so users write the empty case explicitly.
func InvalidDescription(ruleType, description string) string {
	if ruleType == "templated_signal" && description != "" {
		return "description must be an empty string for templated_signal rules."
	}
	if ruleType != "templated_signal" && description == "" {
		return fmt.Sprintf("description must not be empty for a %q rule.", ruleType)
	}
	return ""
}

// actionConstraint describes the action types a rule type may declare, and
// how many actions it must declare.
type actionConstraint struct {
	Types    []string
	MinItems int
	MaxItems int
}

var actionConstraintsByRuleType = map[string]actionConstraint{
	"request":          {Types: []string{"allow", "block", "add_signal", "browser_challenge", "verify_token", "dynamic_challenge", "deception"}, MinItems: 1, MaxItems: 2},
	"signal":           {Types: []string{"exclude_signal"}, MinItems: 1, MaxItems: 1},
	"rate_limit":       {Types: []string{"block_signal", "log_request", "browser_challenge", "dynamic_challenge", "deception"}, MinItems: 1, MaxItems: 1},
	"templated_signal": {Types: []string{"templated_signal"}, MinItems: 1, MaxItems: 1},
}

// ActionCountBounds returns the minimum and maximum number of actions a
// rule of the given type may declare. ok is false if ruleType is not one
// of the four recognized rule types (the schema's own `type` enum
// validator already rejects that case).
func ActionCountBounds(ruleType string) (minItems, maxItems int, ok bool) {
	c, ok := actionConstraintsByRuleType[ruleType]
	return c.MinItems, c.MaxItems, ok
}

// ActionCountOutOfRange reports whether the number of actions violates the
// rule type's minimum/maximum action count.
func ActionCountOutOfRange(ruleType string, actions []ActionModel) bool {
	minItems, maxItems, ok := ActionCountBounds(ruleType)
	if !ok {
		return false
	}
	return len(actions) < minItems || len(actions) > maxItems
}

// InvalidActionTypeIndexes returns the index of every action whose type is
// not valid for the rule's type (e.g. `exclude_signal` is only valid on a
// `signal` rule; `verify_token` is not valid on a `rate_limit` rule).
func InvalidActionTypeIndexes(ruleType string, actions []ActionModel) []int {
	c, ok := actionConstraintsByRuleType[ruleType]
	if !ok {
		return nil
	}

	allowed := make(map[string]bool, len(c.Types))
	for _, t := range c.Types {
		allowed[t] = true
	}

	var indexes []int
	for i, a := range actions {
		if t := a.Type.ValueString(); t != "" && !allowed[t] {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

// actionFields maps each action type to the ActionModel fields the API
// accepts for it; a field missing from a type's list isn't accepted at
// all. actionRequiredFields (below) is the required subset of this same
// map.
var actionFields = map[string][]string{
	"allow":             {},
	"verify_token":      {},
	"block":             {"redirect_url", "response_code"},
	"block_signal":      {"signal", "redirect_url", "response_code"},
	"add_signal":        {"signal"},
	"exclude_signal":    {"signal"},
	"log_request":       {"signal"},
	"templated_signal":  {"signal"},
	"dynamic_challenge": {"signal"},
	"browser_challenge": {"allow_interactive", "signal"},
	"deception":         {"deception_type", "signal"},
}

// actionRequiredFields lists the ActionModel field names that must be set
// for a given action type (e.g. `deception` requires `deception_type`).
// Every field listed here also appears in actionFields[type] - required
// implies allowed.
var actionRequiredFields = map[string][]string{
	"add_signal":        {"signal"},
	"browser_challenge": {"allow_interactive"},
	"deception":         {"deception_type"},
	"exclude_signal":    {"signal"},
	"block_signal":      {"signal"},
	"log_request":       {"signal"},
	"templated_signal":  {"signal"},
}

// MissingRequiredActionFields returns, for each action index, the names of
// required fields that action's type declares but leaves unset.
func MissingRequiredActionFields(actions []ActionModel) map[int][]string {
	missing := map[int][]string{}
	for i, a := range actions {
		var need []string
		for _, field := range actionRequiredFields[a.Type.ValueString()] {
			switch field {
			case "signal":
				if a.Signal.IsNull() || a.Signal.ValueString() == "" {
					need = append(need, "signal")
				}
			case "allow_interactive":
				if a.AllowInteractive.IsNull() {
					need = append(need, "allow_interactive")
				}
			case "deception_type":
				if a.DeceptionType.IsNull() || a.DeceptionType.ValueString() == "" {
					need = append(need, "deception_type")
				}
			}
		}
		if len(need) > 0 {
			missing[i] = need
		}
	}
	return missing
}

// InvalidActionFieldIndexes returns, for each action index, the names of
// ActionModel fields that are set but not defined on that action's type -
// e.g. `redirect_url` on `type = allow`, or `signal` on `type = block` -
// per actionFields above.
func InvalidActionFieldIndexes(actions []ActionModel) map[int][]string {
	invalid := map[int][]string{}
	for i, a := range actions {
		allowed := make(map[string]bool, len(actionFields[a.Type.ValueString()]))
		for _, f := range actionFields[a.Type.ValueString()] {
			allowed[f] = true
		}

		var bad []string
		if !allowed["signal"] && !a.Signal.IsNull() && a.Signal.ValueString() != "" {
			bad = append(bad, "signal")
		}
		if !allowed["allow_interactive"] && !a.AllowInteractive.IsNull() {
			bad = append(bad, "allow_interactive")
		}
		if !allowed["deception_type"] && !a.DeceptionType.IsNull() && a.DeceptionType.ValueString() != "" {
			bad = append(bad, "deception_type")
		}
		if !allowed["redirect_url"] && !a.RedirectURL.IsNull() && a.RedirectURL.ValueString() != "" {
			bad = append(bad, "redirect_url")
		}
		if !allowed["response_code"] && !a.ResponseCode.IsNull() {
			bad = append(bad, "response_code")
		}

		if len(bad) > 0 {
			invalid[i] = bad
		}
	}
	return invalid
}

// clientIdentifierFields maps each client_identifiers type to the
// key/name/signal fields the API accepts for it.
// clientIdentifierRequiredFields (below) is the required subset of this
// same map.
var clientIdentifierFields = map[string][]string{
	"ip":             {},
	"request_header": {"key", "name"},
	"request_cookie": {"name"},
	"post_parameter": {"name"},
	"signal_payload": {"signal"},
}

// clientIdentifierRequiredFields lists the field required for a given
// client_identifiers type.
var clientIdentifierRequiredFields = map[string][]string{
	"request_header": {"name"},
	"request_cookie": {"name"},
	"post_parameter": {"name"},
	"signal_payload": {"signal"},
}

// InvalidClientIdentifiers returns, for each client_identifiers entry, a
// message describing why its type/field combination is invalid: either a
// key/name/signal field is set that its type doesn't define, or a field its
// type requires is left unset. Mirrors InvalidActionFieldIndexes/
// MissingRequiredActionFields' pattern for actions.
func InvalidClientIdentifiers(identifiers []ClientIdentifierModel) []string {
	var issues []string
	for i, ci := range identifiers {
		t := ci.Type.ValueString()
		allowed := make(map[string]bool, len(clientIdentifierFields[t]))
		for _, f := range clientIdentifierFields[t] {
			allowed[f] = true
		}

		hasKey := !ci.Key.IsNull() && ci.Key.ValueString() != ""
		hasName := !ci.Name.IsNull() && ci.Name.ValueString() != ""
		hasSignal := !ci.Signal.IsNull() && ci.Signal.ValueString() != ""

		if !allowed["key"] && hasKey {
			issues = append(issues, fmt.Sprintf("client_identifiers[%d] (type = %q) must not set `key`.", i, t))
		}
		if !allowed["name"] && hasName {
			issues = append(issues, fmt.Sprintf("client_identifiers[%d] (type = %q) must not set `name`.", i, t))
		}
		if !allowed["signal"] && hasSignal {
			issues = append(issues, fmt.Sprintf("client_identifiers[%d] (type = %q) must not set `signal`.", i, t))
		}

		for _, field := range clientIdentifierRequiredFields[t] {
			switch field {
			case "name":
				if !hasName {
					issues = append(issues, fmt.Sprintf("client_identifiers[%d] (type = %q) must set `name`.", i, t))
				}
			case "signal":
				if !hasSignal {
					issues = append(issues, fmt.Sprintf("client_identifiers[%d] (type = %q) must set `signal`.", i, t))
				}
			}
		}
	}
	return issues
}
