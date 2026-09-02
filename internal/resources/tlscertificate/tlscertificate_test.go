package tlscertificate

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly"
)

const testCertPEM = `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIaCP0/EQGH1JoT0S+VuxdDAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTI0MDEwMTAwMDAwMFoXDTM0MDEwMTAwMDAwMFow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABJQ0
5x9C+7hGvj52TQyfvR4iZ5V2b8k1cwx4/rG2b+X3sQyGz3G1U4b5aFhLDvv2Rk6l
1Hn/9d0dqE8gk9dQfWmjTTBLMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAMBgNVHRMBAf8EAjAAMBYGA1UdEQQPMA2CC2V4YW1wbGUuY29tMAoG
CCqGSM49BAMCA0gAMEUCIQDkS0v3sh4Q5eGqoxIQ4uhqzXk1YbTk9y0Kx7VJj0Zw
3wIgVYt2yv1kO8Q7QpXk0h8QeM3nCw2NPRJZLDktQGpVXOc=
-----END CERTIFICATE-----
`

func TestFlattenToModel_full(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)
	cert := &fastly.CustomTLSCertificate{
		ID:                 "cert-123",
		Name:               "example-cert",
		IssuedTo:           "example.com",
		Issuer:             "Acme Co",
		Replace:            true,
		SerialNumber:       "1234",
		SignatureAlgorithm: "SHA256WithRSA",
		Domains: []*fastly.TLSDomain{
			{ID: "example.com"},
			{ID: "*.example.com"},
		},
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}

	m, diags := flattenToModel(context.Background(), cert)
	require.False(t, diags.HasError())

	assert.Equal(t, "cert-123", m.ID.ValueString())
	assert.Equal(t, "example-cert", m.Name.ValueString())
	assert.Equal(t, "example.com", m.IssuedTo.ValueString())
	assert.Equal(t, "Acme Co", m.Issuer.ValueString())
	assert.True(t, m.Replace.ValueBool())
	assert.Equal(t, "1234", m.SerialNumber.ValueString())
	assert.Equal(t, "SHA256WithRSA", m.SignatureAlgorithm.ValueString())
	assert.Equal(t, createdAt.Format(time.RFC3339), m.CreatedAt.ValueString())
	assert.Equal(t, updatedAt.Format(time.RFC3339), m.UpdatedAt.ValueString())

	var domains []string
	assert.False(t, m.Domains.ElementsAs(context.Background(), &domains, false).HasError())
	assert.ElementsMatch(t, []string{"example.com", "*.example.com"}, domains)
}

// No timestamps returned: created_at/updated_at must flatten to null, not "".
func TestFlattenToModel_noTimestamps(t *testing.T) {
	cert := &fastly.CustomTLSCertificate{ID: "cert-123"}

	m, diags := flattenToModel(context.Background(), cert)
	require.False(t, diags.HasError())
	assert.True(t, m.CreatedAt.IsNull())
	assert.True(t, m.UpdatedAt.IsNull())
}

func TestWarnIfReplaceRecommended(t *testing.T) {
	var diags diag.Diagnostics
	warnIfReplaceRecommended(&fastly.CustomTLSCertificate{ID: "cert-123", Replace: true}, &diags)
	assert.Len(t, diags, 1)
	assert.Equal(t, diag.SeverityWarning, diags[0].Severity())
}

func TestWarnIfReplaceRecommended_noWarning(t *testing.T) {
	var diags diag.Diagnostics
	warnIfReplaceRecommended(&fastly.CustomTLSCertificate{ID: "cert-123", Replace: false}, &diags)
	assert.Empty(t, diags)
}

func validatePEMBlocks(t *testing.T, value string) validator.StringResponse {
	t.Helper()
	req := validator.StringRequest{
		Path:        path.Root("certificate_body"),
		ConfigValue: types.StringValue(value),
	}
	resp := validator.StringResponse{}
	pemBlocks{}.ValidateString(context.Background(), req, &resp)
	return resp
}

func TestPEMBlocksValidator_valid(t *testing.T) {
	resp := validatePEMBlocks(t, testCertPEM)
	assert.False(t, resp.Diagnostics.HasError())
}

func TestPEMBlocksValidator_wrongType(t *testing.T) {
	resp := validatePEMBlocks(t, "-----BEGIN PRIVATE KEY-----\nYQ==\n-----END PRIVATE KEY-----\n")
	assert.True(t, resp.Diagnostics.HasError())
}

func TestPEMBlocksValidator_notPEM(t *testing.T) {
	resp := validatePEMBlocks(t, "not a certificate")
	assert.True(t, resp.Diagnostics.HasError())
}

func TestPEMBlocksValidator_nullOrUnknownSkipped(t *testing.T) {
	req := validator.StringRequest{Path: path.Root("certificate_body"), ConfigValue: types.StringNull()}
	resp := validator.StringResponse{}
	pemBlocks{}.ValidateString(context.Background(), req, &resp)
	assert.False(t, resp.Diagnostics.HasError())

	req = validator.StringRequest{Path: path.Root("certificate_body"), ConfigValue: types.StringUnknown()}
	resp = validator.StringResponse{}
	pemBlocks{}.ValidateString(context.Background(), req, &resp)
	assert.False(t, resp.Diagnostics.HasError())
}
