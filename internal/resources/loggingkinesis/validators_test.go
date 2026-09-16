package loggingkinesis

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
)

// TestAuthenticationRequired pins the Fastly API's own constraint — a
// Kinesis endpoint must carry either an IAM role or an access/secret key
// pair — as a plan-time check, since the OpenAPI spec marks all three
// credential fields independently optional/nullable and doesn't surface
// this cross-field requirement (confirmed against the live API returning
// "either an IAM role or an access key and secret key pair must be
// provided" for a credential-less create).
func TestAuthenticationRequired(t *testing.T) {
	tests := []struct {
		name      string
		obj       types.Object
		wantError bool
	}{
		{
			name:      "omitted block resolves to all-empty defaults, which is invalid",
			obj:       types.ObjectNull(authenticationAttributeTypes),
			wantError: true,
		},
		{
			name:      "all three empty",
			obj:       NewAuthenticationObject(types.StringValue(""), types.StringValue(""), types.StringValue("")),
			wantError: true,
		},
		{
			name:      "iam_role only",
			obj:       NewAuthenticationObject(types.StringValue(""), types.StringValue(""), types.StringValue("some-role-arn")),
			wantError: false,
		},
		{
			name:      "access_key and secret_key",
			obj:       NewAuthenticationObject(types.StringValue("access"), types.StringValue("secret"), types.StringValue("")),
			wantError: false,
		},
		{
			name:      "access_key without secret_key",
			obj:       NewAuthenticationObject(types.StringValue("access"), types.StringValue(""), types.StringValue("")),
			wantError: true,
		},
		{
			name:      "secret_key without access_key",
			obj:       NewAuthenticationObject(types.StringValue(""), types.StringValue("secret"), types.StringValue("")),
			wantError: true,
		},
		{
			name:      "iam_role combined with access/secret key pair is not forbidden",
			obj:       NewAuthenticationObject(types.StringValue("access"), types.StringValue("secret"), types.StringValue("some-role-arn")),
			wantError: false,
		},
		{
			name:      "unknown object is skipped, not flagged",
			obj:       types.ObjectUnknown(authenticationAttributeTypes),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.ObjectRequest{
				Path:        path.Root("authentication"),
				ConfigValue: tt.obj,
			}
			resp := &validator.ObjectResponse{}

			authenticationRequired{}.ValidateObject(context.Background(), req, resp)

			assert.Equal(t, tt.wantError, resp.Diagnostics.HasError())
		})
	}
}

func TestValidateNoVCLOnlyAttributesForCompute(t *testing.T) {
	tests := []struct {
		name      string
		format    types.String
		version   types.Int64
		placement types.String
		respCond  types.String
		wantError bool
	}{
		{
			name:      "no VCL-only attributes configured",
			format:    types.StringNull(),
			version:   types.Int64Null(),
			placement: types.StringNull(),
			respCond:  types.StringNull(),
			wantError: false,
		},
		{
			name:      "format configured",
			format:    types.StringValue("custom-format"),
			version:   types.Int64Null(),
			placement: types.StringNull(),
			respCond:  types.StringNull(),
			wantError: true,
		},
		{
			name:      "all VCL-only attributes configured",
			format:    types.StringValue("custom-format"),
			version:   types.Int64Value(2),
			placement: types.StringValue("none"),
			respCond:  types.StringValue("cond"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildTestConfig(t, map[string]schema.Attribute{
				"format":             schema.StringAttribute{Optional: true},
				"format_version":     schema.Int64Attribute{Optional: true},
				"placement":          schema.StringAttribute{Optional: true},
				"response_condition": schema.StringAttribute{Optional: true},
			}, map[string]attr.Value{
				"format":             tt.format,
				"format_version":     tt.version,
				"placement":          tt.placement,
				"response_condition": tt.respCond,
			})

			result := ValidateNoVCLOnlyAttributesForCompute(context.Background(), cfg)

			assert.Equal(t, tt.wantError, result.HasError())
		})
	}
}

// buildTestConfig builds a minimal tfsdk.Config for validators that read
// sibling attributes via Config.GetAttribute. Only the attributes needed by
// the validator under test are included in the schema.
func buildTestConfig(t *testing.T, attrs map[string]schema.Attribute, values map[string]attr.Value) tfsdk.Config {
	t.Helper()
	ctx := context.Background()

	s := schema.Schema{Attributes: attrs}
	objType := s.Type().TerraformType(ctx)

	tfValues := make(map[string]tftypes.Value, len(values))
	for name, v := range values {
		tv, err := v.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("building terraform value for %q: %v", name, err)
		}
		tfValues[name] = tv
	}

	raw := tftypes.NewValue(objType, tfValues)
	return tfsdk.Config{Raw: raw, Schema: s}
}
