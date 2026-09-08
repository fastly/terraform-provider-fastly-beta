package tlsactivation

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestBuildCreateInput_minimal(t *testing.T) {
	plan := Model{
		CertificateID: types.StringValue("cert-123"),
		Domain:        types.StringValue("example.com"),
	}

	input := buildCreateInput(plan)
	assert.Equal(t, "cert-123", input.Certificate.ID)
	assert.Equal(t, "example.com", input.Domain.ID)
	assert.Nil(t, input.Configuration)
}

func TestBuildCreateInput_withConfiguration(t *testing.T) {
	plan := Model{
		CertificateID:   types.StringValue("cert-123"),
		Domain:          types.StringValue("example.com"),
		ConfigurationID: types.StringValue("config-456"),
	}

	input := buildCreateInput(plan)
	if assert.NotNil(t, input.Configuration) {
		assert.Equal(t, "config-456", input.Configuration.ID)
	}
}

func TestBuildUpdateInput_alwaysSendsCertificate(t *testing.T) {
	plan := Model{
		CertificateID: types.StringValue("cert-123"),
	}

	input := buildUpdateInput("activation-1", plan)
	assert.Equal(t, "activation-1", input.ID)
	if assert.NotNil(t, input.Certificate) {
		assert.Equal(t, "cert-123", input.Certificate.ID)
	}
	assert.Nil(t, input.MutualAuthentication)
}

func TestBuildUpdateInput_withMutualAuthentication(t *testing.T) {
	plan := Model{
		CertificateID:          types.StringValue("cert-123"),
		MutualAuthenticationID: types.StringValue("mtls-789"),
	}

	input := buildUpdateInput("activation-1", plan)
	if assert.NotNil(t, input.MutualAuthentication) {
		assert.Equal(t, "mtls-789", input.MutualAuthentication.ID)
	}
}

func TestFlattenToModel_full(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	activation := &fastly.TLSActivation{
		ID:            "activation-1",
		Certificate:   &fastly.CustomTLSCertificate{ID: "cert-123"},
		Configuration: &fastly.TLSConfiguration{ID: "config-456"},
		Domain:        &fastly.TLSDomain{ID: "example.com"},
		CreatedAt:     &createdAt,
		MutualAuthentication: &fastly.TLSMutualAuthentication{
			ID: "mtls-789",
		},
	}

	m := flattenToModel(activation)
	assert.Equal(t, "activation-1", m.ID.ValueString())
	assert.Equal(t, "cert-123", m.CertificateID.ValueString())
	assert.Equal(t, "config-456", m.ConfigurationID.ValueString())
	assert.Equal(t, "example.com", m.Domain.ValueString())
	assert.Equal(t, "mtls-789", m.MutualAuthenticationID.ValueString())
	assert.Equal(t, createdAt.Format(time.RFC3339), m.CreatedAt.ValueString())
}

// No mutual TLS: mutual_authentication_id must flatten to null, not "".
func TestFlattenToModel_noMutualAuthentication(t *testing.T) {
	activation := &fastly.TLSActivation{
		ID:          "activation-1",
		Certificate: &fastly.CustomTLSCertificate{ID: "cert-123"},
		Domain:      &fastly.TLSDomain{ID: "example.com"},
	}

	m := flattenToModel(activation)
	assert.True(t, m.MutualAuthenticationID.IsNull())
	assert.True(t, m.ConfigurationID.IsNull())
	assert.True(t, m.CreatedAt.IsNull())
}
