package tlsactivation

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly"
)

func testActivations() []*fastly.TLSActivation {
	return []*fastly.TLSActivation{
		{
			ID:            "activation-1",
			Certificate:   &fastly.CustomTLSCertificate{ID: "cert-1"},
			Configuration: &fastly.TLSConfiguration{ID: "config-1"},
			Domain:        &fastly.TLSDomain{ID: "one.example.com"},
		},
		{
			ID:            "activation-2",
			Certificate:   &fastly.CustomTLSCertificate{ID: "cert-2"},
			Configuration: &fastly.TLSConfiguration{ID: "config-1"},
			Domain:        &fastly.TLSDomain{ID: "two.example.com"},
		},
	}
}

func TestFilterActivations_byCertificateID(t *testing.T) {
	matches := filterActivations(testActivations(), DataSourceModel{CertificateID: types.StringValue("cert-2")})
	if assert.Len(t, matches, 1) {
		assert.Equal(t, "activation-2", matches[0].ID)
	}
}

func TestFilterActivations_byDomain(t *testing.T) {
	matches := filterActivations(testActivations(), DataSourceModel{Domain: types.StringValue("one.example.com")})
	if assert.Len(t, matches, 1) {
		assert.Equal(t, "activation-1", matches[0].ID)
	}
}

func TestFilterActivations_multipleMatches(t *testing.T) {
	matches := filterActivations(testActivations(), DataSourceModel{ConfigurationID: types.StringValue("config-1")})
	assert.Len(t, matches, 2)
}

func TestFilterActivations_noMatches(t *testing.T) {
	matches := filterActivations(testActivations(), DataSourceModel{CertificateID: types.StringValue("does-not-exist")})
	assert.Empty(t, matches)
}

func TestFlattenToModel(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	activation := &fastly.TLSActivation{
		ID:            "activation-1",
		Certificate:   &fastly.CustomTLSCertificate{ID: "cert-1"},
		Configuration: &fastly.TLSConfiguration{ID: "config-1"},
		Domain:        &fastly.TLSDomain{ID: "one.example.com"},
		CreatedAt:     &createdAt,
	}

	m := flattenToModel(activation)
	assert.Equal(t, "activation-1", m.ID.ValueString())
	assert.Equal(t, "cert-1", m.CertificateID.ValueString())
	assert.Equal(t, "config-1", m.ConfigurationID.ValueString())
	assert.Equal(t, "one.example.com", m.Domain.ValueString())
	assert.Equal(t, createdAt.Format(time.RFC3339), m.CreatedAt.ValueString())
}
