package logging

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
	"github.com/fastly/terraform-provider-fastly-beta/internal/validation"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ValidateConditionReferences rejects a response_condition naming a condition
// block absent from config.
func ValidateConditionReferences[M any](endpoints []M, label string, name, responseCondition func(M) types.String, conditionNames map[string]struct{}) error {
	return validation.References(endpoints, label, name, "response_condition",
		func(m M) []string {
			rc := responseCondition(m)
			if rc.IsUnknown() || rc.IsNull() {
				return nil
			}
			return []string{service.StringValue(rc)}
		},
		"condition", conditionNames)
}
