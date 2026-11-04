package logging

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v19/fastly"
)

func newTestData(t *testing.T, serviceType string, locked bool) *fastlyclient.Data {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/details"):
			_, _ = w.Write([]byte(`{"type":"` + serviceType + `"}`))
		case strings.Contains(r.URL.Path, "/version/"):
			if locked {
				_, _ = w.Write([]byte(`{"locked":true}`))
				return
			}
			_, _ = w.Write([]byte(`{"locked":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := fastly.NewClientForEndpoint("test-key", srv.URL)
	require.NoError(t, err)

	return &fastlyclient.Data{
		Client:         client,
		TypeChecker:    service.NewTypeChecker(client),
		VersionChecker: service.NewVersionChecker(client),
	}
}

func vclOnlyConfig(t *testing.T, format types.String) (map[string]schema.Attribute, map[string]attr.Value) {
	t.Helper()

	attrs := map[string]schema.Attribute{
		"format":             schema.StringAttribute{Optional: true},
		"format_version":     schema.Int64Attribute{Optional: true},
		"placement":          schema.StringAttribute{Optional: true},
		"response_condition": schema.StringAttribute{Optional: true},
	}
	values := map[string]attr.Value{
		"format":             format,
		"format_version":     types.Int64Null(),
		"placement":          types.StringNull(),
		"response_condition": types.StringNull(),
	}
	return attrs, values
}

func TestPrepareWrite(t *testing.T) {
	tests := []struct {
		name        string
		serviceType string
		locked      bool
		format      types.String
		wantOK      bool
		wantType    string
		wantErr     string
	}{
		{name: "vcl service with VCL-only attributes", serviceType: service.TypeVCL, format: types.StringValue("f"), wantOK: true, wantType: service.TypeVCL},
		{name: "compute service without VCL-only attributes", serviceType: service.TypeCompute, format: types.StringNull(), wantOK: true, wantType: service.TypeCompute},
		{name: "compute service rejects VCL-only attributes", serviceType: service.TypeCompute, format: types.StringValue("f"), wantErr: "VCL-only attributes not supported on Compute services"},
		{name: "locked version is rejected", serviceType: service.TypeVCL, locked: true, format: types.StringNull(), wantErr: "Fastly service version is not mutable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := newTestData(t, tt.serviceType, tt.locked)
			attrs, values := vclOnlyConfig(t, tt.format)
			cfg := buildTestConfig(t, attrs, values)

			var diags diag.Diagnostics
			gotType, ok := PrepareWrite(context.Background(), data, "fastly_service_logging_test", "svc", 1, cfg, &diags)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantType, gotType)
			if tt.wantErr == "" {
				assert.False(t, diags.HasError())
				return
			}
			require.True(t, diags.HasError())
			assert.Equal(t, tt.wantErr, diags.Errors()[0].Summary())
		})
	}
}

func TestPrepareDelete(t *testing.T) {
	t.Run("mutable version proceeds", func(t *testing.T) {
		var diags diag.Diagnostics
		ok := PrepareDelete(context.Background(), newTestData(t, service.TypeVCL, false), "fastly_service_logging_test", "svc", 1, &diags)
		assert.True(t, ok)
		assert.False(t, diags.HasError())
	})

	t.Run("locked version stops with an error", func(t *testing.T) {
		var diags diag.Diagnostics
		ok := PrepareDelete(context.Background(), newTestData(t, service.TypeVCL, true), "fastly_service_logging_test", "svc", 1, &diags)
		assert.False(t, ok)
		assert.True(t, diags.HasError())
	})
}
