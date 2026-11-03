package loggingkinesis

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
