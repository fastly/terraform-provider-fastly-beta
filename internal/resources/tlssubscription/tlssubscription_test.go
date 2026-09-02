package tlssubscription

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly"
)

func mustSet(t *testing.T, values ...string) types.Set {
	t.Helper()
	set, diags := types.SetValueFrom(context.Background(), types.StringType, values)
	require.False(t, diags.HasError())
	return set
}

func TestBuildCreateInput_minimal(t *testing.T) {
	plan := Model{
		CertificateAuthority: types.StringValue("lets-encrypt"),
		Domains:              mustSet(t, "example.com"),
		CommonName:           types.StringNull(),
		ConfigurationID:      types.StringNull(),
	}

	input, diags := buildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError())
	assert.Equal(t, "lets-encrypt", input.CertificateAuthority)
	assert.Nil(t, input.Configuration)
	assert.Nil(t, input.CommonName)
	assert.ElementsMatch(t, []*fastly.TLSDomain{{ID: "example.com"}}, input.Domains)
}

func TestBuildCreateInput_withConfigurationAndCommonName(t *testing.T) {
	plan := Model{
		CertificateAuthority: types.StringValue("globalsign"),
		Domains:              mustSet(t, "example.com", "www.example.com"),
		CommonName:           types.StringValue("www.example.com"),
		ConfigurationID:      types.StringValue("config-1"),
	}

	input, diags := buildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError())
	assert.Equal(t, "config-1", input.Configuration.ID)
	assert.Equal(t, "www.example.com", input.CommonName.ID)
}

func TestBuildCreateInput_commonNameNotInDomains(t *testing.T) {
	plan := Model{
		CertificateAuthority: types.StringValue("lets-encrypt"),
		Domains:              mustSet(t, "example.com"),
		CommonName:           types.StringValue("other.com"),
	}

	_, diags := buildCreateInput(context.Background(), plan)
	require.True(t, diags.HasError())
}

func TestBuildUpdateInput_alwaysSendsAllThree(t *testing.T) {
	plan := Model{
		Domains:         mustSet(t, "example.com"),
		CommonName:      types.StringValue("example.com"),
		ConfigurationID: types.StringValue("config-1"),
		ForceUpdate:     types.BoolValue(true),
	}

	input, diags := buildUpdateInput(context.Background(), "sub-1", plan)
	require.False(t, diags.HasError())
	assert.Equal(t, "sub-1", input.ID)
	assert.True(t, input.Force)
	assert.Equal(t, "example.com", input.CommonName.ID)
	assert.Equal(t, "config-1", input.Configuration.ID)
	assert.ElementsMatch(t, []*fastly.TLSDomain{{ID: "example.com"}}, input.Domains)
}

func TestBuildUpdateInput_commonNameNotInDomains(t *testing.T) {
	plan := Model{
		Domains:    mustSet(t, "example.com"),
		CommonName: types.StringValue("other.com"),
	}

	_, diags := buildUpdateInput(context.Background(), "sub-1", plan)
	require.True(t, diags.HasError())
}

func TestFlattenToModel(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)

	// No Certificates set: a non-empty certificate ID would make flattenToModel call
	// ListTLSDomains for the configuration_id override, which needs a live client (covered
	// instead by TestLatestActivationConfigurationID / TestResolveConfigurationID_noCertificate).
	subscription := &fastly.TLSSubscription{
		ID:                   "sub-1",
		CertificateAuthority: "lets-encrypt",
		State:                "issued",
		CommonName:           &fastly.TLSDomain{ID: "example.com"},
		Configuration:        &fastly.TLSConfiguration{ID: "config-1"},
		Domains:              []*fastly.TLSDomain{{ID: "example.com"}, {ID: "www.example.com"}},
		CreatedAt:            &createdAt,
		UpdatedAt:            &updatedAt,
		Authorizations: []*fastly.TLSAuthorizations{
			{
				ID: "auth-1",
				Challenges: []fastly.TLSChallenge{
					{Type: "managed-dns", RecordName: "_acme-challenge.example.com", RecordType: "CNAME", Values: []string{"xxxx.fastly-validations.com"}},
					{Type: "managed-http", RecordName: "example.com", RecordType: "A", Values: []string{"127.0.0.1", "127.0.0.2"}},
				},
			},
		},
	}

	m, diags := flattenToModel(context.Background(), nil, subscription)
	require.False(t, diags.HasError())

	assert.Equal(t, "sub-1", m.ID.ValueString())
	assert.Equal(t, "lets-encrypt", m.CertificateAuthority.ValueString())
	assert.Equal(t, "", m.CertificateID.ValueString())
	assert.Equal(t, "example.com", m.CommonName.ValueString())
	assert.Equal(t, "config-1", m.ConfigurationID.ValueString())
	assert.Equal(t, "issued", m.State.ValueString())
	assert.Equal(t, createdAt.Format(time.RFC3339), m.CreatedAt.ValueString())
	assert.Equal(t, updatedAt.Format(time.RFC3339), m.UpdatedAt.ValueString())

	var domains []string
	require.False(t, m.Domains.ElementsAs(context.Background(), &domains, false).HasError())
	assert.ElementsMatch(t, []string{"example.com", "www.example.com"}, domains)

	assert.Equal(t, 1, len(m.ManagedDNSChallenges.Elements()))
	assert.Equal(t, 1, len(m.ManagedHTTPChallenges.Elements()))
}

func TestFlattenToModel_noTimestampsOrChallenges(t *testing.T) {
	subscription := &fastly.TLSSubscription{ID: "sub-1"}

	m, diags := flattenToModel(context.Background(), nil, subscription)
	require.False(t, diags.HasError())
	assert.True(t, m.CreatedAt.IsNull())
	assert.True(t, m.UpdatedAt.IsNull())
	assert.Equal(t, "", m.CertificateID.ValueString())
	assert.Equal(t, "", m.CommonName.ValueString())
	assert.Equal(t, 0, len(m.ManagedDNSChallenges.Elements()))
	assert.Equal(t, 0, len(m.ManagedHTTPChallenges.Elements()))
}

func TestFlattenToModel_managedDNSChallengeMissingValues(t *testing.T) {
	subscription := &fastly.TLSSubscription{
		ID: "sub-1",
		Authorizations: []*fastly.TLSAuthorizations{
			{Challenges: []fastly.TLSChallenge{{Type: "managed-dns", Values: nil}}},
		},
	}

	_, diags := flattenToModel(context.Background(), nil, subscription)
	assert.True(t, diags.HasError())
}

func TestResolveConfigurationID_noCertificate(t *testing.T) {
	subscription := &fastly.TLSSubscription{Configuration: &fastly.TLSConfiguration{ID: "config-1"}}
	got := resolveConfigurationID(context.Background(), nil, subscription, "")
	assert.Equal(t, "config-1", got)
}

func TestLatestActivationConfigurationID(t *testing.T) {
	domains := []*fastly.TLSDomain{
		{ID: "example.com", Activations: []*fastly.TLSActivation{}},
		{ID: "www.example.com", Activations: []*fastly.TLSActivation{{Configuration: &fastly.TLSConfiguration{ID: "config-2"}}}},
	}
	assert.Equal(t, "config-2", latestActivationConfigurationID(domains, "config-1"))
}

func TestLatestActivationConfigurationID_noneFound(t *testing.T) {
	assert.Equal(t, "config-1", latestActivationConfigurationID(nil, "config-1"))
}

func TestLowercaseString(t *testing.T) {
	req := validator.StringRequest{Path: path.Root("common_name"), ConfigValue: types.StringValue("Example.com")}
	resp := &validator.StringResponse{}
	lowercaseString{}.ValidateString(context.Background(), req, resp)
	assert.True(t, resp.Diagnostics.HasError())
}

func TestLowercaseString_valid(t *testing.T) {
	req := validator.StringRequest{Path: path.Root("common_name"), ConfigValue: types.StringValue("example.com")}
	resp := &validator.StringResponse{}
	lowercaseString{}.ValidateString(context.Background(), req, resp)
	assert.False(t, resp.Diagnostics.HasError())
}

func TestLowercaseSetElements(t *testing.T) {
	req := validator.SetRequest{Path: path.Root("domains"), ConfigValue: mustSet(t, "example.com", "WWW.example.com")}
	resp := &validator.SetResponse{}
	lowercaseSetElements{}.ValidateSet(context.Background(), req, resp)
	assert.True(t, resp.Diagnostics.HasError())
}

func TestLowercaseSetElements_valid(t *testing.T) {
	req := validator.SetRequest{Path: path.Root("domains"), ConfigValue: mustSet(t, "example.com", "www.example.com")}
	resp := &validator.SetResponse{}
	lowercaseSetElements{}.ValidateSet(context.Background(), req, resp)
	assert.False(t, resp.Diagnostics.HasError())
}

// stateWithSubscriptionState builds a real tfsdk.State (via the resource's own schema) with its
// "state" attribute set to value, for exercising plan modifiers that read prior state.
func stateWithSubscriptionState(t *testing.T, value string) tfsdk.State {
	t.Helper()

	var res Resource
	var schemaResp resource.SchemaResponse
	res.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	state := tfsdk.State{Schema: schemaResp.Schema}
	m := Model{
		ID:                    types.StringValue("sub-1"),
		CertificateAuthority:  types.StringValue("lets-encrypt"),
		CertificateID:         types.StringValue("cert-1"),
		CommonName:            types.StringValue("example.com"),
		ConfigurationID:       types.StringValue("config-1"),
		CreatedAt:             types.StringValue("2024-01-01T00:00:00Z"),
		Domains:               mustSet(t, "example.com"),
		ForceDestroy:          types.BoolValue(false),
		ForceUpdate:           types.BoolValue(false),
		ManagedDNSChallenges:  types.SetValueMust(types.ObjectType{AttrTypes: managedDNSChallengeAttrTypes}, []attr.Value{}),
		ManagedHTTPChallenges: types.SetValueMust(types.ObjectType{AttrTypes: managedHTTPChallengeAttrTypes}, []attr.Value{}),
		State:                 types.StringValue(value),
		UpdatedAt:             types.StringValue("2024-01-01T00:00:00Z"),
	}
	diags := state.Set(context.Background(), &m)
	require.False(t, diags.HasError(), diags)
	return state
}

func TestSubscriptionIsMutable_noPriorState(t *testing.T) {
	var nullState tfsdk.State
	assert.True(t, subscriptionIsMutable(context.Background(), nullState))
}

func TestSubscriptionIsMutable(t *testing.T) {
	assert.True(t, subscriptionIsMutable(context.Background(), stateWithSubscriptionState(t, "issued")))
	assert.True(t, subscriptionIsMutable(context.Background(), stateWithSubscriptionState(t, "pending")))
	assert.False(t, subscriptionIsMutable(context.Background(), stateWithSubscriptionState(t, "processing")))
	assert.False(t, subscriptionIsMutable(context.Background(), stateWithSubscriptionState(t, "renewing")))
}

func TestRequiresReplaceUnlessMutableString_noChange(t *testing.T) {
	req := planmodifier.StringRequest{
		StateValue: types.StringValue("example.com"),
		PlanValue:  types.StringValue("example.com"),
	}
	resp := &planmodifier.StringResponse{}
	requiresReplaceUnlessMutableString{}.PlanModifyString(context.Background(), req, resp)
	assert.False(t, resp.RequiresReplace)
}

func TestRequiresReplaceUnlessMutableString_forcesReplaceWhenImmutable(t *testing.T) {
	req := planmodifier.StringRequest{
		StateValue: types.StringValue("example.com"),
		PlanValue:  types.StringValue("other.com"),
		State:      stateWithSubscriptionState(t, "processing"),
	}
	resp := &planmodifier.StringResponse{}
	requiresReplaceUnlessMutableString{}.PlanModifyString(context.Background(), req, resp)
	assert.True(t, resp.RequiresReplace)
}

func TestRequiresReplaceUnlessMutableSet_allowsInPlaceWhenIssued(t *testing.T) {
	req := planmodifier.SetRequest{
		StateValue: mustSet(t, "example.com"),
		PlanValue:  mustSet(t, "example.com", "www.example.com"),
		State:      stateWithSubscriptionState(t, "issued"),
	}
	resp := &planmodifier.SetResponse{}
	requiresReplaceUnlessMutableSet{}.PlanModifySet(context.Background(), req, resp)
	assert.False(t, resp.RequiresReplace)
}

func TestRequiresReplaceUnlessMutableSet_forcesReplaceWhenImmutable(t *testing.T) {
	req := planmodifier.SetRequest{
		StateValue: mustSet(t, "example.com"),
		PlanValue:  mustSet(t, "example.com", "www.example.com"),
		State:      stateWithSubscriptionState(t, "processing"),
	}
	resp := &planmodifier.SetResponse{}
	requiresReplaceUnlessMutableSet{}.PlanModifySet(context.Background(), req, resp)
	assert.True(t, resp.RequiresReplace)
}
