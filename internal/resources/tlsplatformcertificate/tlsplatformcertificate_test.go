package tlsplatformcertificate

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/fastly/go-fastly/v17/fastly"
)

const (
	testLeafCert = `-----BEGIN CERTIFICATE-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQA=
-----END CERTIFICATE-----`
	testRootCert = `-----BEGIN CERTIFICATE-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQB=
-----END CERTIFICATE-----`
	testPrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQC=
-----END PRIVATE KEY-----`
)

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)

	assert.Equal(t, "fastly_tls_platform_certificate", resp.TypeName)
}

func TestResourceAttributes_allowUntrustedRootDefault(t *testing.T) {
	attrs := ResourceAttributes()

	attr, ok := attrs["allow_untrusted_root"].(resourceschema.BoolAttribute)
	assert.True(t, ok)

	var resp defaults.BoolResponse
	attr.Default.DefaultBool(context.Background(), defaults.BoolRequest{}, &resp)
	assert.Equal(t, types.BoolValue(false), resp.PlanValue)
}

func TestPemBlockValidator(t *testing.T) {
	v := pemBlockValidator{pemType: "CERTIFICATE"}

	cases := []struct {
		name  string
		value string
		valid bool
	}{
		{"valid single block", testLeafCert, true},
		{"trailing whitespace is not a second block", testLeafCert + "\n", true},
		{"multiple blocks", testLeafCert + "\n" + testRootCert, false},
		{"wrong type", testPrivateKeyPEM, false},
		{"not pem", "not a pem block", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validator.StringRequest{ConfigValue: types.StringValue(c.value)}
			resp := &validator.StringResponse{}
			v.ValidateString(context.Background(), req, resp)
			assert.Equal(t, c.valid, !resp.Diagnostics.HasError())
		})
	}
}

func TestPemBlockValidator_nullOrUnknownSkipped(t *testing.T) {
	v := pemBlockValidator{pemType: "CERTIFICATE"}

	for _, value := range []types.String{types.StringNull(), types.StringUnknown()} {
		req := validator.StringRequest{ConfigValue: value}
		resp := &validator.StringResponse{}
		v.ValidateString(context.Background(), req, resp)
		assert.False(t, resp.Diagnostics.HasError())
	}
}

func TestPemBlocksValidator(t *testing.T) {
	v := pemBlocksValidator{pemType: "CERTIFICATE"}

	cases := []struct {
		name  string
		value string
		valid bool
	}{
		{"single block", testLeafCert, true},
		{"multiple blocks", testLeafCert + "\n" + testRootCert, true},
		{"wrong type", testPrivateKeyPEM, false},
		{"not pem", "not a pem block", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validator.StringRequest{ConfigValue: types.StringValue(c.value)}
			resp := &validator.StringResponse{}
			v.ValidateString(context.Background(), req, resp)
			assert.Equal(t, c.valid, !resp.Diagnostics.HasError())
		})
	}
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		CertificateBody:    types.StringValue(testLeafCert),
		IntermediatesBlob:  types.StringValue(testRootCert),
		ConfigurationID:    types.StringValue("config-abc"),
		AllowUntrustedRoot: types.BoolValue(true),
	}

	input := BuildCreateInput(plan)
	assert.Equal(t, testLeafCert, input.CertBlob)
	assert.Equal(t, testRootCert, input.IntermediatesBlob)
	assert.True(t, input.AllowUntrusted)
	assert.Len(t, input.Configurations, 1)
	assert.Equal(t, "config-abc", input.Configurations[0].ID)
}

func TestBuildUpdateInput(t *testing.T) {
	plan := Model{
		CertificateBody:    types.StringValue(testLeafCert),
		IntermediatesBlob:  types.StringValue(testRootCert),
		AllowUntrustedRoot: types.BoolValue(false),
	}

	input := BuildUpdateInput("cert-123", plan)
	assert.Equal(t, "cert-123", input.ID)
	assert.Equal(t, testLeafCert, input.CertBlob)
	assert.Equal(t, testRootCert, input.IntermediatesBlob)
	assert.False(t, input.AllowUntrusted)
}

func TestFlattenToModel(t *testing.T) {
	notBefore := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	certificate := &fastly.BulkCertificate{
		ID:        "cert-123",
		NotBefore: &notBefore,
		NotAfter:  &notAfter,
		Replace:   false,
		Configurations: []*fastly.TLSConfiguration{
			{ID: "config-abc"},
		},
		Domains: []*fastly.TLSDomain{
			{ID: "example.com"},
			{ID: "*.example.com"},
		},
	}

	m, diags := FlattenToModel(context.Background(), certificate)
	assert.False(t, diags.HasError())
	assert.Empty(t, diags)
	assert.Equal(t, types.StringValue("cert-123"), m.ID)
	assert.Equal(t, types.StringValue("config-abc"), m.ConfigurationID)
	assert.Equal(t, types.StringValue(notBefore.Format(time.RFC3339)), m.NotBefore)
	assert.Equal(t, types.StringValue(notAfter.Format(time.RFC3339)), m.NotAfter)
	assert.Equal(t, types.BoolValue(false), m.Replace)

	var domains []string
	diags = m.Domains.ElementsAs(context.Background(), &domains, false)
	assert.False(t, diags.HasError())
	assert.ElementsMatch(t, []string{"example.com", "*.example.com"}, domains)
}

func TestFlattenToModel_missingConfigurationDoesNotPanic(t *testing.T) {
	certificate := &fastly.BulkCertificate{ID: "cert-123"}

	m, diags := FlattenToModel(context.Background(), certificate)
	assert.False(t, diags.HasError())
	assert.Equal(t, types.StringValue(""), m.ConfigurationID)
	assert.Equal(t, types.StringNull(), m.NotBefore)
	assert.Equal(t, types.StringNull(), m.NotAfter)
}

func TestFlattenToModel_replaceWarns(t *testing.T) {
	certificate := &fastly.BulkCertificate{ID: "cert-123", Replace: true}

	m, diags := FlattenToModel(context.Background(), certificate)
	assert.False(t, diags.HasError())
	assert.True(t, diags.WarningsCount() > 0)
	assert.Equal(t, types.BoolValue(true), m.Replace)
}

func TestCarryForwardWriteOnly(t *testing.T) {
	src := Model{
		CertificateBody:    types.StringValue(testLeafCert),
		IntermediatesBlob:  types.StringValue(testRootCert),
		AllowUntrustedRoot: types.BoolValue(true),
	}
	dst := Model{ID: types.StringValue("cert-123")}

	carryForwardWriteOnly(&dst, src)

	assert.Equal(t, types.StringValue(testLeafCert), dst.CertificateBody)
	assert.Equal(t, types.StringValue(testRootCert), dst.IntermediatesBlob)
	assert.Equal(t, types.BoolValue(true), dst.AllowUntrustedRoot)
	assert.Equal(t, types.StringValue("cert-123"), dst.ID)
}
