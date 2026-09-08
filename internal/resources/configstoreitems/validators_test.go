package configstoreitems

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestValidItemsValidator(t *testing.T) {
	tests := []struct {
		name        string
		items       map[string]attr.Value
		wantError   bool
		errorDetail string
	}{
		{
			name: "valid boundaries",
			items: map[string]attr.Value{
				strings.Repeat("k", maxItemKeyLength): types.StringValue(strings.Repeat("v", maxItemValueLength)),
			},
		},
		{
			name: "empty key",
			items: map[string]attr.Value{
				"": types.StringValue("value"),
			},
			wantError:   true,
			errorDetail: "between 1 and 256 characters",
		},
		{
			name: "key too long",
			items: map[string]attr.Value{
				strings.Repeat("k", maxItemKeyLength+1): types.StringValue("value"),
			},
			wantError:   true,
			errorDetail: "between 1 and 256 characters",
		},
		{
			name: "value too long",
			items: map[string]attr.Value{
				"key": types.StringValue(strings.Repeat("v", maxItemValueLength+1)),
			},
			wantError:   true,
			errorDetail: "at most 8000 characters",
		},
		{
			name: "unicode limits count characters not bytes",
			items: map[string]attr.Value{
				strings.Repeat("é", maxItemKeyLength): types.StringValue(strings.Repeat("界", maxItemValueLength)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, valueDiags := types.MapValue(types.StringType, tt.items)
			require.False(t, valueDiags.HasError())

			req := validator.MapRequest{ConfigValue: value}
			var resp validator.MapResponse
			ValidItems().ValidateMap(context.Background(), req, &resp)

			require.Equal(t, tt.wantError, resp.Diagnostics.HasError())
			if tt.errorDetail != "" {
				var details strings.Builder
				for _, d := range resp.Diagnostics {
					if details.Len() > 0 {
						details.WriteString("\n")
					}
					details.WriteString(d.Detail())
				}
				require.Contains(t, details.String(), tt.errorDetail)
			}
		})
	}
}

func TestValidItemsValidatorIgnoresNullAndUnknownMaps(t *testing.T) {
	tests := []struct {
		name  string
		value types.Map
	}{
		{
			name:  "null map",
			value: types.MapNull(types.StringType),
		},
		{
			name:  "unknown map",
			value: types.MapUnknown(types.StringType),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.MapRequest{ConfigValue: tt.value}
			var resp validator.MapResponse

			ValidItems().ValidateMap(context.Background(), req, &resp)

			require.False(t, resp.Diagnostics.HasError())
		})
	}
}
