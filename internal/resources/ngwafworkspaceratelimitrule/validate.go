package ngwafworkspaceratelimitrule

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// clientIdentifierFields maps each client_identifiers type to the
// key/name/signal fields the API accepts for it.
// clientIdentifierRequiredFields (below) is the required subset of this same
// map.
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
// type requires is left unset.
func InvalidClientIdentifiers(identifiers []ClientIdentifierModel) []string {
	var issues []string
	for i, ci := range identifiers {
		t := ci.Type.ValueString()
		allowed := make(map[string]bool, len(clientIdentifierFields[t]))
		for _, f := range clientIdentifierFields[t] {
			allowed[f] = true
		}

		hasKey := isSet(ci.Key)
		hasName := isSet(ci.Name)
		hasSignal := isSet(ci.Signal)

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

// isSet treats an unknown value as set, so a field whose value is still
// being computed never draws a diagnostic against something the user did not
// write. The API gets the final say on those.
func isSet(v types.String) bool {
	return v.IsUnknown() || (!v.IsNull() && v.ValueString() != "")
}
