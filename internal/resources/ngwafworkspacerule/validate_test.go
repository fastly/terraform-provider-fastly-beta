package ngwafworkspacerule

import (
	"testing"

	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestInvalidDescription(t *testing.T) {
	assert.Equal(t, "description must be an empty string for templated_signal rules.", InvalidDescription("templated_signal", "not empty"))
	assert.Empty(t, InvalidDescription("templated_signal", ""), "empty description is required and valid for templated_signal")
	assert.Empty(t, InvalidDescription("request", "Block a specific IP"), "non-empty description is valid for request")
	assert.Equal(t, "description must not be empty for a \"request\" rule.", InvalidDescription("request", ""))
	assert.Equal(t, "description must not be empty for a \"signal\" rule.", InvalidDescription("signal", ""))
	assert.Equal(t, "description must not be empty for a \"rate_limit\" rule.", InvalidDescription("rate_limit", ""))
}

func TestTotalConditionCount(t *testing.T) {
	conditions := make([]ngwafrule.ConditionModel, 6)
	groups := make([]ngwafrule.GroupConditionModel, 2)
	multivals := make([]ngwafrule.MultivalConditionModel, 2)

	assert.Equal(t, 10, TotalConditionCount(conditions, groups, multivals), "6+2+2 = 10, at the limit")
	assert.LessOrEqual(t, TotalConditionCount(conditions, groups, multivals), MaxConditions)

	conditions = append(conditions, ngwafrule.ConditionModel{})
	assert.Equal(t, 11, TotalConditionCount(conditions, groups, multivals), "one over the limit")
	assert.Greater(t, TotalConditionCount(conditions, groups, multivals), MaxConditions)

	// A group_condition's own nested conditions don't count toward the
	// combined total - only the group_condition entry itself (already
	// counted above) does.
	groups[0].Condition = make([]ngwafrule.ConditionModel, 20)
	assert.Equal(t, 11, TotalConditionCount(conditions, groups, multivals), "nested group conditions aren't counted")
}

func TestInvalidActionTypeIndexes(t *testing.T) {
	actions := []ActionModel{
		{Type: types.StringValue("block")},
		{Type: types.StringValue("exclude_signal")},
		{Type: types.StringValue("verify_token")},
	}

	assert.Equal(t, []int{1}, InvalidActionTypeIndexes("request", actions), "exclude_signal is signal-only")
	assert.Nil(t, InvalidActionTypeIndexes("signal", []ActionModel{{Type: types.StringValue("exclude_signal")}}))
	assert.Equal(t, []int{0, 1, 2}, InvalidActionTypeIndexes("rate_limit", actions), "block/exclude_signal/verify_token are all invalid on rate_limit")
	assert.Nil(t, InvalidActionTypeIndexes("not_a_real_type", actions), "unrecognized rule type is left to the schema's own type enum validator")
}

func TestActionCountOutOfRange(t *testing.T) {
	one := []ActionModel{{Type: types.StringValue("allow")}}
	two := []ActionModel{{Type: types.StringValue("allow")}, {Type: types.StringValue("block")}}
	three := []ActionModel{{Type: types.StringValue("allow")}, {Type: types.StringValue("block")}, {Type: types.StringValue("add_signal")}}

	assert.False(t, ActionCountOutOfRange("request", one))
	assert.False(t, ActionCountOutOfRange("request", two), "request rules allow up to 2 actions")
	assert.True(t, ActionCountOutOfRange("request", three), "request rules allow at most 2 actions")
	assert.True(t, ActionCountOutOfRange("signal", two), "signal rules allow exactly 1 action")
	assert.False(t, ActionCountOutOfRange("unknown_type", three), "unrecognized rule type has no bounds to violate")
}

func TestMissingRequiredActionFields(t *testing.T) {
	actions := []ActionModel{
		{Type: types.StringValue("add_signal")},                                   // missing signal
		{Type: types.StringValue("add_signal"), Signal: types.StringValue("sig")}, // ok
		{Type: types.StringValue("browser_challenge")},                            // missing allow_interactive
		{Type: types.StringValue("deception")},                                    // missing deception_type
		{Type: types.StringValue("allow")},                                        // no required fields
	}

	missing := MissingRequiredActionFields(actions)

	assert.Equal(t, []string{"signal"}, missing[0])
	_, ok := missing[1]
	assert.False(t, ok, "action with signal set should not be reported missing")
	assert.Equal(t, []string{"allow_interactive"}, missing[2])
	assert.Equal(t, []string{"deception_type"}, missing[3])
	_, ok = missing[4]
	assert.False(t, ok, "allow has no required companion fields")
}

func TestInvalidActionFieldIndexes(t *testing.T) {
	actions := []ActionModel{
		{Type: types.StringValue("allow"), RedirectURL: types.StringValue("https://example.com/")},                                      // redirect_url not allowed
		{Type: types.StringValue("block"), RedirectURL: types.StringValue("https://example.com/"), ResponseCode: types.Int64Value(301)}, // ok
		{Type: types.StringValue("block"), Signal: types.StringValue("sig")},                                                            // signal not allowed
		{Type: types.StringValue("verify_token")},                                                                                       // ok, no fields
		{Type: types.StringValue("deception"), DeceptionType: types.StringValue("ato"), AllowInteractive: types.BoolValue(true)},        // allow_interactive not allowed
	}

	invalid := InvalidActionFieldIndexes(actions)

	assert.Equal(t, []string{"redirect_url"}, invalid[0])
	_, ok := invalid[1]
	assert.False(t, ok, "block with redirect_url/response_code is valid")
	assert.Equal(t, []string{"signal"}, invalid[2])
	_, ok = invalid[3]
	assert.False(t, ok, "verify_token has no type-specific fields set")
	assert.Equal(t, []string{"allow_interactive"}, invalid[4])
}

func TestInvalidClientIdentifiers(t *testing.T) {
	identifiers := []ClientIdentifierModel{
		{Type: types.StringValue("ip")},                                                    // ok
		{Type: types.StringValue("signal_payload")},                                        // missing signal
		{Type: types.StringValue("signal_payload"), Signal: types.StringValue("site.sig")}, // ok
		{Type: types.StringValue("signal_payload"), Signal: types.StringValue("site.sig"), Key: types.StringValue("k"), Name: types.StringValue("n")}, // key+name not allowed
		{Type: types.StringValue("request_header"), Name: types.StringValue("X-Foo"), Signal: types.StringValue("site.sig")},                          // signal not allowed
		{Type: types.StringValue("request_header"), Key: types.StringValue("k1")},                                                                     // missing required name
		{Type: types.StringValue("request_header"), Key: types.StringValue("k1"), Name: types.StringValue("X-Foo")},                                   // ok, key is valid here
		{Type: types.StringValue("request_cookie"), Name: types.StringValue("session_id")},                                                            // ok
		{Type: types.StringValue("request_cookie"), Key: types.StringValue("k1"), Name: types.StringValue("session_id")},                              // key not allowed
		{Type: types.StringValue("post_parameter")},                                                                                                   // missing required name
		{Type: types.StringValue("ip"), Name: types.StringValue("nope")},                                                                              // name not allowed
	}

	issues := InvalidClientIdentifiers(identifiers)

	assert.Contains(t, issues, "client_identifiers[1] (type = \"signal_payload\") must set `signal`.")
	assert.Contains(t, issues, "client_identifiers[3] (type = \"signal_payload\") must not set `key`.")
	assert.Contains(t, issues, "client_identifiers[3] (type = \"signal_payload\") must not set `name`.")
	assert.Contains(t, issues, "client_identifiers[4] (type = \"request_header\") must not set `signal`.")
	assert.Contains(t, issues, "client_identifiers[5] (type = \"request_header\") must set `name`.")
	assert.Contains(t, issues, "client_identifiers[8] (type = \"request_cookie\") must not set `key`.")
	assert.Contains(t, issues, "client_identifiers[9] (type = \"post_parameter\") must set `name`.")
	assert.Contains(t, issues, "client_identifiers[10] (type = \"ip\") must not set `name`.")
	assert.Len(t, issues, 8, "identifiers[0], [2], [6], and [7] are valid and should not be reported")
}
