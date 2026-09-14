package loggingelasticsearch

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestHTTPSURL(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantError bool
	}{
		{"valid https URL", "https://elasticsearch.example.com", false},
		{"http scheme rejected", "http://elasticsearch.example.com", true},
		{"missing host", "https://", true},
		{"not a URL", "not a url", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			httpsURL{}.ValidateString(context.Background(), validator.StringRequest{
				Path:        path.Root("url"),
				ConfigValue: types.StringValue(tt.value),
			}, resp)

			assert.Equal(t, tt.wantError, resp.Diagnostics.HasError())
		})
	}
}

func TestTrimmedValidator(t *testing.T) {
	tests := []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{"trimmed value", types.StringValue("-----BEGIN CERTIFICATE-----"), false},
		{"empty value", types.StringValue(""), false},
		{"leading whitespace", types.StringValue(" cert"), true},
		{"trailing whitespace", types.StringValue("cert\n"), true},
		{"null is not validated", types.StringNull(), false},
		{"unknown is not validated", types.StringUnknown(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.StringRequest{
				Path:        path.Root("tls").AtName("ca_cert"),
				ConfigValue: tt.value,
			}
			resp := &validator.StringResponse{}
			trimmedValidator{}.ValidateString(context.Background(), req, resp)

			assert.Equal(t, tt.wantError, resp.Diagnostics.HasError())
		})
	}
}
