package auditlogeventmapping

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v18/fastly/notifications/v1/eventmappings"
)

func FlattenStringSet(ctx context.Context, values []string) (types.Set, diag.Diagnostics) {
	if values == nil {
		values = []string{}
	}
	return types.SetValueFrom(ctx, types.StringType, values)
}

// FlattenToModel maps an API mapping onto Model. The API returns an empty
// `scope_ids` and `description` for unset values, so those keep a null prior
// value rather than becoming an empty set/string, which would read as drift.
func FlattenToModel(ctx context.Context, m *eventmappings.EventMapping, prior Model) (Model, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := Model{
		ID:            types.StringValue(m.ID),
		Name:          types.StringValue(m.Name),
		Description:   prior.Description,
		ScopeType:     types.StringValue(m.ScopeType),
		MappingStatus: types.StringValue(m.MappingStatus),
		CreatedAt:     types.StringValue(m.CreatedAt.Format(time.RFC3339)),
		UpdatedAt:     types.StringValue(m.UpdatedAt.Format(time.RFC3339)),
	}

	if m.Description != "" || !prior.Description.IsNull() {
		out.Description = types.StringValue(m.Description)
	}

	var d diag.Diagnostics
	out.EventTypes, d = FlattenStringSet(ctx, m.EventTypes)
	diags.Append(d...)
	out.IntegrationIDs, d = FlattenStringSet(ctx, m.IntegrationIDs)
	diags.Append(d...)

	out.ScopeIDs = types.SetNull(types.StringType)
	if len(m.ScopeIDs) > 0 || (!prior.ScopeIDs.IsNull() && !prior.ScopeIDs.IsUnknown()) {
		out.ScopeIDs, d = FlattenStringSet(ctx, m.ScopeIDs)
		diags.Append(d...)
	}

	return out, diags
}
