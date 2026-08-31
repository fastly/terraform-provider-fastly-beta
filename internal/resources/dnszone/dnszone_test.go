package dnszone

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly/dns/v1/dnszones"
)

func TestBuildCreateInput_minimal(t *testing.T) {
	plan := Model{
		Name:        types.StringValue("example.com."),
		Description: types.StringNull(),
	}

	input := BuildCreateInput(plan)
	assert.Equal(t, "example.com.", *input.Name)
	assert.Equal(t, "secondary", *input.Type)
	assert.Nil(t, input.Description)
	assert.Nil(t, input.XfrConfigInbound)
}

// TestBuildCreateInput_explicitEmptyDescription confirms an explicit empty
// string (distinct from omitted/null) is still sent — the Plugin Framework
// can tell them apart, unlike the legacy provider's SDKv2 zero-value check.
func TestBuildCreateInput_explicitEmptyDescription(t *testing.T) {
	plan := Model{
		Name:        types.StringValue("example.com."),
		Description: types.StringValue(""),
	}

	input := BuildCreateInput(plan)
	assert.NotNil(t, input.Description)
	assert.Equal(t, "", *input.Description)
}

func TestBuildCreateInput_withXfrConfigInbound(t *testing.T) {
	plan := Model{
		Name:        types.StringValue("example.com."),
		Description: types.StringValue("a zone"),
		XfrConfigInbound: []XfrConfigInboundModel{
			{
				InboundTSIGKeyID: types.StringValue("tsig-123"),
				Primaries: []PrimaryModel{
					{Address: types.StringValue("1.2.3.4"), Description: types.StringValue("primary")},
				},
			},
		},
	}

	input := BuildCreateInput(plan)
	assert.Equal(t, "a zone", *input.Description)
	if assert.NotNil(t, input.XfrConfigInbound) {
		assert.Equal(t, `"tsig-123"`, mustMarshalNullable(t, input.XfrConfigInbound.InboundTSIGKeyID))
		if assert.Len(t, input.XfrConfigInbound.Primaries, 1) {
			assert.Equal(t, "1.2.3.4", *input.XfrConfigInbound.Primaries[0].Address)
			assert.Equal(t, "primary", *input.XfrConfigInbound.Primaries[0].Description)
		}
	}
}

// TestBuildCreateInput_emptyInboundTSIGKeyIDOmitted mirrors the legacy
// provider: an empty inbound_tsig_key_id is never sent, only a populated one.
func TestBuildCreateInput_emptyInboundTSIGKeyIDOmitted(t *testing.T) {
	plan := Model{
		Name: types.StringValue("example.com."),
		XfrConfigInbound: []XfrConfigInboundModel{
			{InboundTSIGKeyID: types.StringNull()},
		},
	}

	input := BuildCreateInput(plan)
	assert.Nil(t, input.XfrConfigInbound.InboundTSIGKeyID)
}

// TestBuildUpdateInput_clearedInboundTSIGKeyIDSendsExplicitNull confirms
// Update sends an explicit JSON null to clear inbound_tsig_key_id — omitting
// it (as Create does) leaves the API's prior value in place, since PATCH
// treats an absent field as "don't touch this".
func TestBuildUpdateInput_clearedInboundTSIGKeyIDSendsExplicitNull(t *testing.T) {
	state := Model{
		XfrConfigInbound: []XfrConfigInboundModel{
			{InboundTSIGKeyID: types.StringValue("tsig-123")},
		},
	}
	plan := Model{
		XfrConfigInbound: []XfrConfigInboundModel{
			{InboundTSIGKeyID: types.StringNull()},
		},
	}

	input := BuildUpdateInput("zone-id", plan, state)
	assert.Equal(t, "null", mustMarshalNullable(t, input.XfrConfigInbound.InboundTSIGKeyID))
}

func TestBuildUpdateInput_descriptionUnchangedNotSent(t *testing.T) {
	state := Model{Description: types.StringValue("same")}
	plan := Model{Description: types.StringValue("same")}

	input := BuildUpdateInput("zone-id", plan, state)
	assert.Equal(t, "zone-id", *input.ZoneID)
	assert.Nil(t, input.Description)
}

func TestBuildUpdateInput_descriptionChangedSent(t *testing.T) {
	state := Model{Description: types.StringValue("old")}
	plan := Model{Description: types.StringValue("new")}

	input := BuildUpdateInput("zone-id", plan, state)
	assert.Equal(t, `"new"`, mustMarshalNullable(t, input.Description))
}

// TestBuildUpdateInput_xfrConfigRemovedNotSent documents a known gap from the
// legacy provider: removing xfr_config_inbound entirely doesn't clear it
// server-side — the API has no way to clear the whole object.
func TestBuildUpdateInput_xfrConfigRemovedNotSent(t *testing.T) {
	state := Model{
		XfrConfigInbound: []XfrConfigInboundModel{
			{InboundTSIGKeyID: types.StringValue("tsig-123")},
		},
	}
	plan := Model{}

	input := BuildUpdateInput("zone-id", plan, state)
	assert.Nil(t, input.XfrConfigInbound)
}

func TestFlattenToModel_noXfrConfig(t *testing.T) {
	zone := &dnszones.Zone{
		ID:   new("zone-id"),
		Name: new("example.com."),
	}

	m := FlattenToModel(zone)
	assert.Equal(t, types.StringValue("zone-id"), m.ID)
	assert.Equal(t, types.StringValue("example.com."), m.Name)
	assert.Equal(t, types.StringNull(), m.Description)
	assert.Nil(t, m.XfrConfigInbound)
}

// TestFlattenToModel_xfrConfigWithNoContentOmitted mirrors the legacy
// provider: an XfrConfigInbound with no TSIG key and no primaries is treated
// as absent, not an empty block.
func TestFlattenToModel_xfrConfigWithNoContentOmitted(t *testing.T) {
	zone := &dnszones.Zone{
		ID:               new("zone-id"),
		Name:             new("example.com."),
		XfrConfigInbound: &dnszones.XfrConfigInbound{},
	}

	m := FlattenToModel(zone)
	assert.Nil(t, m.XfrConfigInbound)
}

func TestFlattenToModel_withXfrConfig(t *testing.T) {
	zone := &dnszones.Zone{
		ID:          new("zone-id"),
		Name:        new("example.com."),
		Description: new("a zone"),
		XfrConfigInbound: &dnszones.XfrConfigInbound{
			InboundTSIGKeyID: new("tsig-123"),
			Primaries: []dnszones.Primary{
				{Address: new("1.2.3.4"), Description: new("primary")},
			},
		},
	}

	m := FlattenToModel(zone)
	assert.Equal(t, types.StringValue("a zone"), m.Description)
	if assert.Len(t, m.XfrConfigInbound, 1) {
		assert.Equal(t, types.StringValue("tsig-123"), m.XfrConfigInbound[0].InboundTSIGKeyID)
		if assert.Len(t, m.XfrConfigInbound[0].Primaries, 1) {
			assert.Equal(t, types.StringValue("1.2.3.4"), m.XfrConfigInbound[0].Primaries[0].Address)
			assert.Equal(t, types.StringValue("primary"), m.XfrConfigInbound[0].Primaries[0].Description)
		}
	}
}

func TestNameValidator(t *testing.T) {
	attrs := ResourceAttributes()
	nameAttr, ok := attrs["name"].(schema.StringAttribute)
	assert.True(t, ok)

	cases := []struct {
		value string
		valid bool
	}{
		{"example.com.", true},
		{"example.com", false},
		{"", false},
	}

	for _, c := range cases {
		req := validator.StringRequest{ConfigValue: types.StringValue(c.value)}
		resp := &validator.StringResponse{}
		for _, v := range nameAttr.Validators {
			v.ValidateString(context.Background(), req, resp)
		}
		assert.Equal(t, c.valid, !resp.Diagnostics.HasError(), "value %q", c.value)
	}
}

// TestIPv4AddressValidator guards a documented API constraint: DNS zone
// transfers don't support IPv6 for primary servers, so this must catch a bad
// value here rather than at apply.
func TestIPv4AddressValidator(t *testing.T) {
	v := ipv4AddressValidator{}

	cases := []struct {
		value string
		valid bool
	}{
		{"1.2.3.4", true},
		{"255.255.255.255", true},
		{"", true}, // unconfigured/optional, not this validator's concern
		{"::1", false},
		{"2001:db8::1", false},
		{"not-an-ip", false},
		{"example.com", false},
		{"1.2.3.4.5", false},
	}

	for _, c := range cases {
		req := validator.StringRequest{ConfigValue: types.StringValue(c.value)}
		resp := &validator.StringResponse{}
		v.ValidateString(context.Background(), req, resp)
		assert.Equal(t, c.valid, !resp.Diagnostics.HasError(), "value %q", c.value)
	}
}

func mustMarshalNullable(t *testing.T, n interface{ MarshalJSON() ([]byte, error) }) string {
	t.Helper()
	b, err := n.MarshalJSON()
	assert.NoError(t, err)
	return string(b)
}

// TestReconcileDescription_emptyCollapsedToNullIsRestored guards a live API
// quirk: an explicit empty description is stored as absent and read back as
// null, which fails Terraform's consistency check without this
// reconciliation.
func TestReconcileDescription_emptyCollapsedToNullIsRestored(t *testing.T) {
	got := ReconcileDescription(types.StringNull(), types.StringValue(""))
	assert.Equal(t, types.StringValue(""), got)
}

// TestReconcileDescription_genuineDriftIsNotMasked confirms real drift isn't
// masked: a non-empty known value turning null is genuine external drift and
// must be surfaced.
func TestReconcileDescription_genuineDriftIsNotMasked(t *testing.T) {
	got := ReconcileDescription(types.StringNull(), types.StringValue("a real description"))
	assert.Equal(t, types.StringNull(), got)
}

// TestReconcileDescription_nonNullReturnedIsUnchanged confirms a non-null API
// value is always trusted, regardless of what was known before.
func TestReconcileDescription_nonNullReturnedIsUnchanged(t *testing.T) {
	got := ReconcileDescription(types.StringValue("from api"), types.StringValue(""))
	assert.Equal(t, types.StringValue("from api"), got)
}
