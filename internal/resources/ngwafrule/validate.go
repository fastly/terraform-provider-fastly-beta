package ngwafrule

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// AppliesToWildcard is the applies_to entry that scopes an account rule to
// every workspace in the account rather than to named ones.
const AppliesToWildcard = "*"

// appliesToWildcardExclusive rejects an applies_to set pairing the wildcard
// with named workspace IDs. The API accepts such a set with a 201 but
// normalizes it to just ["*"], so the value read back never matches the one
// configured - Terraform would fail the apply it just made with "Provider
// produced inconsistent result after apply". Rejecting at plan time turns that
// into a clear message, and costs nothing: the wildcard already covers every
// workspace the named IDs would have.
type appliesToWildcardExclusive struct{}

func (v appliesToWildcardExclusive) Description(_ context.Context) string {
	return fmt.Sprintf("`%s` must be the only entry when present", AppliesToWildcard)
}

func (v appliesToWildcardExclusive) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v appliesToWildcardExclusive) ValidateSet(_ context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	elements := req.ConfigValue.Elements()
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || len(elements) < 2 {
		return
	}

	for _, e := range elements {
		s, ok := e.(types.String)
		if !ok || s.IsNull() || s.IsUnknown() {
			continue
		}
		if s.ValueString() == AppliesToWildcard {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Conflicting applies_to entries",
				fmt.Sprintf("`%s` already applies the rule to every workspace in the account, so it cannot be combined with named workspace IDs.", AppliesToWildcard),
			)
			return
		}
	}
}

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
	validateActions(actions, actionFields, diags)
}

// ValidateAccountActions is ValidateActions for account-scoped rules, whose
// action set excludes the custom-response fields.
func ValidateAccountActions(actions []AccountActionModel, diags *diag.Diagnostics) {
	converted := make([]ActionModel, 0, len(actions))
	for _, a := range actions {
		converted = append(converted, a.toActionModel())
	}
	validateActions(converted, accountActionFields, diags)
}

func validateActions(actions []ActionModel, allowedFields map[string][]string, diags *diag.Diagnostics) {
	for i, missing := range MissingRequiredActionFields(actions) {
		diags.AddAttributeError(
			path.Root("action").AtListIndex(i),
			"Missing required action field",
			fmt.Sprintf("action[%d] (type = %q) must set: %s.", i, actions[i].Type.ValueString(), strings.Join(missing, ", ")),
		)
	}

	for i, invalid := range InvalidActionFieldIndexes(actions, allowedFields) {
		diags.AddAttributeError(
			path.Root("action").AtListIndex(i),
			"Invalid action field for type",
			fmt.Sprintf("action[%d] (type = %q) must not set: %s.", i, actions[i].Type.ValueString(), strings.Join(invalid, ", ")),
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

// customResponseFields configure a custom response - a redirect target or an
// explicit status code. The account rules endpoint rejects them outright
// ("Validation failed - corp rules cannot use custom responses", confirmed
// against the live API), so account-scoped rules neither expose nor accept
// them, even though AccountRequestRuleBody references the same Rule.Action.*
// components the workspace bodies do.
var customResponseFields = []string{"redirect_url", "response_code"}

// accountActionFields is actionFields as it applies at account scope. Because
// it drives both the rendered attributes and the validators, dropping a field
// here removes it from the schema and the documentation together.
var accountActionFields = actionFieldsWithout(customResponseFields)

func actionFieldsWithout(exclude []string) map[string][]string {
	result := make(map[string][]string, len(actionFields))
	for actionType, fields := range actionFields {
		kept := make([]string, 0, len(fields))
		for _, field := range fields {
			if !slices.Contains(exclude, field) {
				kept = append(kept, field)
			}
		}
		result[actionType] = kept
	}
	return result
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
// per allowedFields, which is actionFields at workspace scope and
// accountActionFields at account scope.
func InvalidActionFieldIndexes(actions []ActionModel, allowedFields map[string][]string) map[int][]string {
	invalid := map[int][]string{}
	for i, a := range actions {
		allowed := make(map[string]bool, len(allowedFields[a.Type.ValueString()]))
		for _, f := range allowedFields[a.Type.ValueString()] {
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
