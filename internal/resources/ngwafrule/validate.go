package ngwafrule

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// MaxConditions caps the combined count of top-level
// condition/group_condition/multival_condition entries. A group_condition or
// multival_condition counts as one entry regardless of how many conditions
// it nests inside.
const MaxConditions = 10

// TotalConditionCount returns the combined number of top-level condition,
// group_condition, and multival_condition entries a rule declares.
func TotalConditionCount(conditions []ConditionModel, groups []GroupConditionModel, multivals []MultivalConditionModel) int {
	return len(conditions) + len(groups) + len(multivals)
}

// ValidateConditions runs the condition checks every rule type shares: at
// least one condition, no more than MaxConditions combined, and no
// group_condition that matches nothing.
//
// Templated signal rules are each individually predefined, so whether a
// condition is required isn't governed by one flat rule across rule types.
// TestAccFastlyNGWAFWorkspaceTemplatedSignalRule_apiRequiresConditions pins
// the live API's behavior for this rule type: it rejects a templated_signal
// rule with no conditions, so this validator applies the same requirement
// here as for every other rule type.
func ValidateConditions(m CommonModel, diags *diag.Diagnostics) {
	if !HasAnyCondition(m.Condition, m.GroupCondition, m.MultivalCondition) {
		diags.AddError(
			"Missing rule conditions",
			"A rule must define at least one 'condition', 'group_condition', or 'multival_condition'.",
		)
	}

	if total := TotalConditionCount(m.Condition, m.GroupCondition, m.MultivalCondition); total > MaxConditions {
		diags.AddError(
			"Too many conditions",
			fmt.Sprintf("a rule may define at most %d combined 'condition'/'group_condition'/'multival_condition' entries, got %d.", MaxConditions, total),
		)
	}

	for _, i := range EmptyGroupConditionIndexes(m.GroupCondition) {
		diags.AddAttributeError(
			path.Root("group_condition").AtListIndex(i),
			"Empty group_condition",
			fmt.Sprintf("group_condition[%d] must define at least one 'condition' or 'multival_condition' block.", i),
		)
	}
}

// ValidateActions runs the per-action field checks for rule types that use
// ActionBlock. The action `type` enum and the action count are already
// enforced by the schema's own validators.
func ValidateActions(actions []ActionModel, diags *diag.Diagnostics) {
	for i, fields := range MissingRequiredActionFields(actions) {
		diags.AddAttributeError(
			path.Root("action").AtListIndex(i),
			"Missing required action field",
			fmt.Sprintf("action[%d] (type = %q) must set: %s.", i, actions[i].Type.ValueString(), strings.Join(fields, ", ")),
		)
	}

	for i, fields := range InvalidActionFieldIndexes(actions) {
		diags.AddAttributeError(
			path.Root("action").AtListIndex(i),
			"Invalid action field for type",
			fmt.Sprintf("action[%d] (type = %q) must not set: %s.", i, actions[i].Type.ValueString(), strings.Join(fields, ", ")),
		)
	}
}

// actionFields maps each action type to the ActionModel fields the API
// accepts for it; a field missing from a type's list isn't accepted at all.
// actionRequiredFields (below) is the required subset of this same map. Both
// also drive the field descriptions in ActionBlock.
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
				if stringAbsent(a.Signal) {
					need = append(need, "signal")
				}
			case "allow_interactive":
				if boolAbsent(a.AllowInteractive) {
					need = append(need, "allow_interactive")
				}
			case "deception_type":
				if stringAbsent(a.DeceptionType) {
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
		if !allowed["signal"] && stringPresent(a.Signal) {
			bad = append(bad, "signal")
		}
		if !allowed["allow_interactive"] && boolPresent(a.AllowInteractive) {
			bad = append(bad, "allow_interactive")
		}
		if !allowed["deception_type"] && stringPresent(a.DeceptionType) {
			bad = append(bad, "deception_type")
		}
		if !allowed["redirect_url"] && stringPresent(a.RedirectURL) {
			bad = append(bad, "redirect_url")
		}
		if !allowed["response_code"] && int64Present(a.ResponseCode) {
			bad = append(bad, "response_code")
		}

		if len(bad) > 0 {
			invalid[i] = bad
		}
	}
	return invalid
}

// The helpers below split "unset" into present/absent rather than
// testing IsNull alone: an unknown value is neither, and treating it as
// either produces a false diagnostic against a value the user never wrote.
// Unknown counts as present when checking a forbidden field and as
// not-absent when checking a required one, so the API gets the final say.

func stringPresent(v types.String) bool {
	return v.IsUnknown() || (!v.IsNull() && v.ValueString() != "")
}

func stringAbsent(v types.String) bool {
	return !v.IsUnknown() && (v.IsNull() || v.ValueString() == "")
}

func boolPresent(v types.Bool) bool {
	return v.IsUnknown() || !v.IsNull()
}

func boolAbsent(v types.Bool) bool {
	return !v.IsUnknown() && v.IsNull()
}

func int64Present(v types.Int64) bool {
	return v.IsUnknown() || !v.IsNull()
}
