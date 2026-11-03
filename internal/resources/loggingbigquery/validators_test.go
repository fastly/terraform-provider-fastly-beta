package loggingbigquery

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestAuthenticationEitherOr(t *testing.T) {
	tests := []struct {
		name                 string
		accountName          types.String
		email                types.String
		secretKey            types.String
		envAccount           string
		envDeprecatedAccount string
		envEmail             string
		envSecret            string
		wantError            bool
	}{
		{
			name:        "account_name only",
			accountName: types.StringValue("svc-account"),
			email:       types.StringNull(),
			secretKey:   types.StringNull(),
			wantError:   false,
		},
		{
			name:        "email and secret_key",
			accountName: types.StringNull(),
			email:       types.StringValue("a@b.com"),
			secretKey:   types.StringValue("secret"),
			wantError:   false,
		},
		{
			name:        "all three set",
			accountName: types.StringValue("svc-account"),
			email:       types.StringValue("a@b.com"),
			secretKey:   types.StringValue("secret"),
			wantError:   false,
		},
		{
			name:        "email only",
			accountName: types.StringNull(),
			email:       types.StringValue("a@b.com"),
			secretKey:   types.StringNull(),
			wantError:   true,
		},
		{
			name:        "secret_key only",
			accountName: types.StringNull(),
			email:       types.StringNull(),
			secretKey:   types.StringValue("secret"),
			wantError:   true,
		},
		{
			name:        "nothing configured, no env vars",
			accountName: types.StringNull(),
			email:       types.StringNull(),
			secretKey:   types.StringNull(),
			wantError:   true,
		},
		{
			name:        "nothing configured, account_name env var set",
			accountName: types.StringNull(),
			email:       types.StringNull(),
			secretKey:   types.StringNull(),
			envAccount:  "svc-account",
			wantError:   false,
		},
		{
			name:                 "nothing configured, deprecated account_name env var set",
			accountName:          types.StringNull(),
			email:                types.StringNull(),
			secretKey:            types.StringNull(),
			envDeprecatedAccount: "legacy-svc-account",
			wantError:            false,
		},
		{
			name:        "email configured, secret_key falls back to env var",
			accountName: types.StringNull(),
			email:       types.StringValue("a@b.com"),
			secretKey:   types.StringNull(),
			envSecret:   "secret",
			wantError:   false,
		},
		{
			name:        "email configured, secret_key unset and no env var",
			accountName: types.StringNull(),
			email:       types.StringValue("a@b.com"),
			secretKey:   types.StringNull(),
			wantError:   true,
		},
		{
			// account_name explicitly "" is configured, not omitted — Terraform's
			// schema Default never overrides it with the env var, so the validator
			// must not treat it as satisfying auth just because the env var is set.
			name:        "account_name explicitly blank does not fall back to env var",
			accountName: types.StringValue(""),
			email:       types.StringNull(),
			secretKey:   types.StringNull(),
			envAccount:  "svc-account",
			wantError:   true,
		},
		{
			// Same as above for email/secret_key: an explicit "" is configured, so
			// it must not fall back to the env var either.
			name:        "email explicitly blank does not fall back to env var",
			accountName: types.StringNull(),
			email:       types.StringValue(""),
			secretKey:   types.StringValue("secret"),
			envEmail:    "a@b.com",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME", tt.envAccount)
			t.Setenv("FASTLY_GCS_ACCOUNT_NAME", tt.envDeprecatedAccount)
			t.Setenv("FASTLY_BQ_EMAIL", tt.envEmail)
			t.Setenv("FASTLY_BQ_SECRET_KEY", tt.envSecret)

			req := validator.ObjectRequest{
				Path:        path.Root("authentication"),
				ConfigValue: NewAuthenticationObject(tt.accountName, tt.email, tt.secretKey),
			}
			resp := &validator.ObjectResponse{}
			authenticationEitherOr{}.ValidateObject(context.Background(), req, resp)

			assert.Equal(t, tt.wantError, resp.Diagnostics.HasError())
		})
	}
}
