package planmodifiers

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestCaseInsensitiveState_PlanModifyString(t *testing.T) {
	tests := []struct {
		name     string
		state    types.String
		config   types.String
		expected types.String
	}{
		{
			name:     "config differs only in case from state",
			state:    types.StringValue("pass"),
			config:   types.StringValue("PASS"),
			expected: types.StringValue("pass"),
		},
		{
			name:     "config matches state exactly",
			state:    types.StringValue("pass"),
			config:   types.StringValue("pass"),
			expected: types.StringValue("pass"),
		},
		{
			name:     "config is a different value",
			state:    types.StringValue("pass"),
			config:   types.StringValue("lookup"),
			expected: types.StringValue("lookup"),
		},
		{
			name:     "no prior state",
			state:    types.StringNull(),
			config:   types.StringValue("PASS"),
			expected: types.StringValue("PASS"),
		},
		{
			name:     "unknown config value",
			state:    types.StringValue("pass"),
			config:   types.StringUnknown(),
			expected: types.StringUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				StateValue:  tt.state,
				ConfigValue: tt.config,
			}
			resp := &planmodifier.StringResponse{PlanValue: tt.config}

			CaseInsensitiveState().PlanModifyString(context.Background(), req, resp)

			assert.Equal(t, tt.expected, resp.PlanValue)
		})
	}
}
