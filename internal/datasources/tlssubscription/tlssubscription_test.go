package tlssubscription

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly"
)

func testSubscriptions() []*fastly.TLSSubscription {
	return []*fastly.TLSSubscription{
		{
			ID:                   "sub-1",
			CertificateAuthority: "lets-encrypt",
			Configuration:        &fastly.TLSConfiguration{ID: "config-1"},
			Domains:              []*fastly.TLSDomain{{ID: "one.example.com"}, {ID: "two.example.com"}},
		},
		{
			ID:                   "sub-2",
			CertificateAuthority: "globalsign",
			Configuration:        &fastly.TLSConfiguration{ID: "config-2"},
			Domains:              []*fastly.TLSDomain{{ID: "three.example.com"}},
		},
	}
}

func mustSet(t *testing.T, values ...string) types.Set {
	t.Helper()
	set, diags := types.SetValueFrom(context.Background(), types.StringType, values)
	require.False(t, diags.HasError())
	return set
}

func TestFilterSubscriptions_byConfigurationID(t *testing.T) {
	matches, diags := filterSubscriptions(context.Background(), testSubscriptions(), DataSourceModel{ConfigurationID: types.StringValue("config-2")})
	require.False(t, diags.HasError())
	if assert.Len(t, matches, 1) {
		assert.Equal(t, "sub-2", matches[0].ID)
	}
}

func TestFilterSubscriptions_byCertificateAuthority(t *testing.T) {
	matches, diags := filterSubscriptions(context.Background(), testSubscriptions(), DataSourceModel{CertificateAuthority: types.StringValue("lets-encrypt")})
	require.False(t, diags.HasError())
	if assert.Len(t, matches, 1) {
		assert.Equal(t, "sub-1", matches[0].ID)
	}
}

func TestFilterSubscriptions_byDomainsSubset(t *testing.T) {
	matches, diags := filterSubscriptions(context.Background(), testSubscriptions(), DataSourceModel{Domains: mustSet(t, "one.example.com")})
	require.False(t, diags.HasError())
	if assert.Len(t, matches, 1) {
		assert.Equal(t, "sub-1", matches[0].ID)
	}
}

func TestFilterSubscriptions_noMatches(t *testing.T) {
	matches, diags := filterSubscriptions(context.Background(), testSubscriptions(), DataSourceModel{CertificateAuthority: types.StringValue("certainly")})
	require.False(t, diags.HasError())
	assert.Empty(t, matches)
}

func TestContainsAll(t *testing.T) {
	assert.True(t, containsAll([]string{"a", "b", "c"}, []string{"a", "c"}))
	assert.False(t, containsAll([]string{"a", "b"}, []string{"a", "c"}))
	assert.True(t, containsAll([]string{"a"}, nil))
}

func TestFlattenToModel(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)
	subscription := &fastly.TLSSubscription{
		ID:                   "sub-1",
		CertificateAuthority: "lets-encrypt",
		State:                "issued",
		CommonName:           &fastly.TLSDomain{ID: "one.example.com"},
		Configuration:        &fastly.TLSConfiguration{ID: "config-1"},
		Domains:              []*fastly.TLSDomain{{ID: "one.example.com"}, {ID: "two.example.com"}},
		Certificates:         []*fastly.TLSSubscriptionCertificate{{ID: "cert-1"}},
		CreatedAt:            &createdAt,
		UpdatedAt:            &updatedAt,
	}

	m, diags := flattenToModel(subscription)
	require.False(t, diags.HasError())

	assert.Equal(t, "sub-1", m.ID.ValueString())
	assert.Equal(t, "lets-encrypt", m.CertificateAuthority.ValueString())
	assert.Equal(t, "one.example.com", m.CommonName.ValueString())
	assert.Equal(t, "config-1", m.ConfigurationID.ValueString())
	assert.Equal(t, "issued", m.State.ValueString())
	assert.Equal(t, createdAt.Format(time.RFC3339), m.CreatedAt.ValueString())
	assert.Equal(t, updatedAt.Format(time.RFC3339), m.UpdatedAt.ValueString())

	var certIDs []string
	require.False(t, m.CertificateIDs.ElementsAs(context.Background(), &certIDs, false).HasError())
	assert.Equal(t, []string{"cert-1"}, certIDs)

	var domains []string
	require.False(t, m.Domains.ElementsAs(context.Background(), &domains, false).HasError())
	assert.ElementsMatch(t, []string{"one.example.com", "two.example.com"}, domains)
}

func TestFlattenToModel_noTimestampsOrCertificate(t *testing.T) {
	subscription := &fastly.TLSSubscription{ID: "sub-1"}

	m, diags := flattenToModel(subscription)
	require.False(t, diags.HasError())
	assert.True(t, m.CreatedAt.IsNull())
	assert.True(t, m.UpdatedAt.IsNull())
	assert.Equal(t, "", m.CommonName.ValueString())
	assert.Equal(t, "", m.ConfigurationID.ValueString())
	assert.Equal(t, 0, len(m.CertificateIDs.Elements()))
}
