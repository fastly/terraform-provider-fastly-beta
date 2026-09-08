package dnszone

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/dns/v1/dnszones"
)

// zoneType is the only value the API accepts for a zone's type.
const zoneType = "secondary"

// buildXfrConfigInboundInput builds the shared xfr_config_inbound payload.
// clearTSIGKeyIfEmpty sends an explicit JSON null for an empty
// inbound_tsig_key_id on update (needed to actually clear it); create just
// omits it, since there's nothing to clear yet.
func buildXfrConfigInboundInput(m XfrConfigInboundModel, clearTSIGKeyIfEmpty bool) *dnszones.XfrConfigInboundInput {
	input := &dnszones.XfrConfigInboundInput{}

	if v := service.StringValue(m.InboundTSIGKeyID); v != "" {
		input.InboundTSIGKeyID = fastly.NewNullable(v)
	} else if clearTSIGKeyIfEmpty {
		input.InboundTSIGKeyID = fastly.NullValue[string]()
	}

	for _, p := range m.Primaries {
		input.Primaries = append(input.Primaries, dnszones.Primary{
			Address:     new(service.StringValue(p.Address)),
			Description: new(service.StringValue(p.Description)),
		})
	}

	return input
}

func BuildCreateInput(plan Model) *dnszones.CreateInput {
	input := &dnszones.CreateInput{
		Name: new(service.StringValue(plan.Name)),
		Type: new(zoneType),
	}

	if !plan.Description.IsNull() {
		input.Description = new(service.StringValue(plan.Description))
	}

	if len(plan.XfrConfigInbound) == 1 {
		input.XfrConfigInbound = buildXfrConfigInboundInput(plan.XfrConfigInbound[0], false)
	}

	return input
}

// BuildUpdateInput never sends xfr_config_inbound once it's removed from
// config entirely: the API has no way to clear the whole object, only
// individual fields within it.
func BuildUpdateInput(zoneID string, plan, state Model) *dnszones.UpdateInput {
	input := &dnszones.UpdateInput{
		ZoneID: new(zoneID),
	}

	if !plan.Description.Equal(state.Description) {
		input.Description = fastly.NewNullable(service.StringValue(plan.Description))
	}

	if len(plan.XfrConfigInbound) == 1 {
		input.XfrConfigInbound = buildXfrConfigInboundInput(plan.XfrConfigInbound[0], true)
	}

	return input
}
