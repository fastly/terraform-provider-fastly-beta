package tlsdomain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly"
)

func testDomains() []*fastly.TLSDomain {
	return []*fastly.TLSDomain{
		{ID: "one.example.com"},
		{ID: "two.example.com"},
	}
}

func TestMatchDomains_match(t *testing.T) {
	matches := matchDomains(testDomains(), "one.example.com")
	if assert.Len(t, matches, 1) {
		assert.Equal(t, "one.example.com", matches[0].ID)
	}
}

func TestMatchDomains_noMatch(t *testing.T) {
	matches := matchDomains(testDomains(), "does-not-exist.example.com")
	assert.Empty(t, matches)
}

func TestMatchDomains_multipleMatches(t *testing.T) {
	domains := append(testDomains(), &fastly.TLSDomain{ID: "one.example.com"})
	matches := matchDomains(domains, "one.example.com")
	assert.Len(t, matches, 2)
}

func TestFlattenToModel(t *testing.T) {
	domain := &fastly.TLSDomain{
		ID: "example.com",
		Activations: []*fastly.TLSActivation{
			{ID: "activation-1"},
		},
		Certificates: []*fastly.CustomTLSCertificate{
			{ID: "cert-1"},
			{ID: "cert-2"},
		},
		Subscriptions: []*fastly.TLSSubscription{
			{ID: "sub-1"},
		},
	}

	m, diags := flattenToModel(context.Background(), domain)
	assert.False(t, diags.HasError(), diags)

	assert.Equal(t, "example.com", m.ID.ValueString())
	assert.Equal(t, "example.com", m.Domain.ValueString())

	var activationIDs, certificateIDs, subscriptionIDs []string
	assert.False(t, m.TLSActivationIDs.ElementsAs(context.Background(), &activationIDs, false).HasError())
	assert.False(t, m.TLSCertificateIDs.ElementsAs(context.Background(), &certificateIDs, false).HasError())
	assert.False(t, m.TLSSubscriptionIDs.ElementsAs(context.Background(), &subscriptionIDs, false).HasError())

	assert.ElementsMatch(t, []string{"activation-1"}, activationIDs)
	assert.ElementsMatch(t, []string{"cert-1", "cert-2"}, certificateIDs)
	assert.ElementsMatch(t, []string{"sub-1"}, subscriptionIDs)
}

func TestFlattenToModelNoRelations(t *testing.T) {
	domain := &fastly.TLSDomain{ID: "example.com"}

	m, diags := flattenToModel(context.Background(), domain)
	assert.False(t, diags.HasError(), diags)

	assert.Empty(t, m.TLSActivationIDs.Elements())
	assert.Empty(t, m.TLSCertificateIDs.Elements())
	assert.Empty(t, m.TLSSubscriptionIDs.Elements())
}
