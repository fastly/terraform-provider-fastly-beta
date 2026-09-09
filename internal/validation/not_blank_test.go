package validation

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestNotBlank(t *testing.T) {
	t.Run("null config value is fine", func(t *testing.T) {
		req := validator.StringRequest{ConfigValue: types.StringNull()}
		var resp validator.StringResponse
		NotBlank("format").(notBlank).ValidateString(context.Background(), req, &resp)
		assert.False(t, resp.Diagnostics.HasError())
	})

	t.Run("unknown config value is fine", func(t *testing.T) {
		req := validator.StringRequest{ConfigValue: types.StringUnknown()}
		var resp validator.StringResponse
		NotBlank("format").(notBlank).ValidateString(context.Background(), req, &resp)
		assert.False(t, resp.Diagnostics.HasError())
	})

	t.Run("non-empty config value is fine", func(t *testing.T) {
		req := validator.StringRequest{ConfigValue: types.StringValue("custom format")}
		var resp validator.StringResponse
		NotBlank("format").(notBlank).ValidateString(context.Background(), req, &resp)
		assert.False(t, resp.Diagnostics.HasError())
	})

	t.Run("explicit empty string is rejected", func(t *testing.T) {
		req := validator.StringRequest{ConfigValue: types.StringValue("")}
		var resp validator.StringResponse
		NotBlank("format").(notBlank).ValidateString(context.Background(), req, &resp)
		if assert.True(t, resp.Diagnostics.HasError()) {
			assert.Contains(t, resp.Diagnostics[0].Summary(), "Invalid `format`")
			assert.Contains(t, resp.Diagnostics[0].Detail(), "cannot be explicitly set to an empty string")
		}
	})
}
