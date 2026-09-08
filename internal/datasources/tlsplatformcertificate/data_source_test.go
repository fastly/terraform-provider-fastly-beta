package tlsplatformcertificate

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestMetadata(t *testing.T) {
	d := NewDataSource()

	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)

	assert.Equal(t, "fastly_tls_platform_certificate", resp.TypeName)
}

func TestMatchesDomains(t *testing.T) {
	certificate := &fastly.BulkCertificate{
		Domains: []*fastly.TLSDomain{
			{ID: "example.com"},
			{ID: "*.example.com"},
		},
	}

	assert.True(t, matchesDomains(certificate, nil), "empty filter matches everything")
	assert.True(t, matchesDomains(certificate, []string{"example.com"}))
	assert.True(t, matchesDomains(certificate, []string{"nope.com", "*.example.com"}))
	assert.False(t, matchesDomains(certificate, []string{"nope.com"}))
}

func TestDomainsFilter(t *testing.T) {
	// The omitted-filter case (null) already converts cleanly via ElementsAs
	// on its own, since []string is nilable - this pins that, and pins the
	// unknown case, which ElementsAs cannot convert and must be checked for
	// explicitly instead.
	nullDiags, diags := domainsFilter(context.Background(), types.SetNull(types.StringType))
	assert.False(t, diags.HasError())
	assert.Nil(t, nullDiags)

	unknownDomains, diags := domainsFilter(context.Background(), types.SetUnknown(types.StringType))
	assert.False(t, diags.HasError())
	assert.Nil(t, unknownDomains)

	known, diags := types.SetValueFrom(context.Background(), types.StringType, []string{"example.com"})
	assert.False(t, diags.HasError())

	knownDomains, diags := domainsFilter(context.Background(), known)
	assert.False(t, diags.HasError())
	assert.Equal(t, []string{"example.com"}, knownDomains)
}

func TestFlattenToDataSourceModel(t *testing.T) {
	notBefore := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	certificate := &fastly.BulkCertificate{
		ID:        "cert-123",
		NotBefore: &notBefore,
		NotAfter:  &notAfter,
		Configurations: []*fastly.TLSConfiguration{
			{ID: "config-abc"},
		},
		Domains: []*fastly.TLSDomain{{ID: "example.com"}},
	}

	m, diags := flattenToDataSourceModel(context.Background(), certificate)
	assert.False(t, diags.HasError())
	assert.Equal(t, types.StringValue("cert-123"), m.ID)
	assert.Equal(t, types.StringValue("config-abc"), m.ConfigurationID)
	assert.Equal(t, types.StringValue(notBefore.Format(time.RFC3339)), m.NotBefore)
	assert.Equal(t, types.StringValue(notAfter.Format(time.RFC3339)), m.NotAfter)

	var domains []string
	diags = m.Domains.ElementsAs(context.Background(), &domains, false)
	assert.False(t, diags.HasError())
	assert.Equal(t, []string{"example.com"}, domains)
}

func TestFlattenToDataSourceModel_missingConfiguration(t *testing.T) {
	certificate := &fastly.BulkCertificate{ID: "cert-123"}

	m, diags := flattenToDataSourceModel(context.Background(), certificate)
	assert.False(t, diags.HasError())
	assert.Equal(t, types.StringValue(""), m.ConfigurationID)
	assert.Equal(t, types.StringNull(), m.NotBefore)
	assert.Equal(t, types.StringNull(), m.NotAfter)
}
