package dnszone

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/dns/v1/dnszones"
)

func flattenXfrConfigInbound(xfr *dnszones.XfrConfigInbound) []XfrConfigInboundModel {
	if xfr == nil || (xfr.InboundTSIGKeyID == nil && len(xfr.Primaries) == 0) {
		return nil
	}

	m := XfrConfigInboundModel{
		InboundTSIGKeyID: service.StringPointerOrNull(xfr.InboundTSIGKeyID),
	}

	for _, p := range xfr.Primaries {
		m.Primaries = append(m.Primaries, PrimaryModel{
			Address:     service.StringPointerOrNull(p.Address),
			Description: service.StringPointerOrNull(p.Description),
		})
	}

	return []XfrConfigInboundModel{m}
}

func FlattenToModel(zone *dnszones.Zone) Model {
	return Model{
		ID:               service.StringPointerOrNull(zone.ID),
		Name:             service.StringPointerOrNull(zone.Name),
		Description:      service.StringPointerOrNull(zone.Description),
		XfrConfigInbound: flattenXfrConfigInbound(zone.XfrConfigInbound),
	}
}

// ReconcileDescription restores an empty description when the API echoes
// null instead: the API stores description = "" as absent and always reads
// it back as null, which would otherwise fail Terraform's consistency check
// or show a perpetual diff. Any other null-vs-known mismatch is genuine
// drift and is left alone.
func ReconcileDescription(returned, known types.String) types.String {
	if returned.IsNull() && known.Equal(types.StringValue("")) {
		return types.StringValue("")
	}
	return returned
}
