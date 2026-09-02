package integration

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

// description isn't always echoed back by the API, so merge over prior instead of
// replacing. The API only ever returns non-sensitive config fields, so the returned
// config always merges into `config`; `authentication` is never round-tripped and is
// carried over from prior state/plan unchanged.
func FlattenToModel(ctx context.Context, i *fastly.Integration, prior Model) (Model, diag.Diagnostics) {
	var diags diag.Diagnostics

	m := Model{
		ID:             types.StringValue(fastly.ToValue(i.ID)),
		Name:           types.StringValue(fastly.ToValue(i.Name)),
		Type:           types.StringValue(fastly.ToValue(i.Type)),
		Description:    prior.Description,
		Authentication: prior.Authentication,
	}
	if i.Description != nil {
		m.Description = types.StringValue(*i.Description)
	}

	config, d := mergeConfig(ctx, prior.Config, i.Config)
	diags.Append(d...)
	m.Config = config

	return m, diags
}

func mergeConfig(ctx context.Context, prior types.Map, remote map[string]string) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics

	if remote == nil {
		return prior, diags
	}

	merged := map[string]attr.Value{}
	if !prior.IsNull() && !prior.IsUnknown() {
		var priorMap map[string]string
		diags.Append(prior.ElementsAs(ctx, &priorMap, false)...)
		for k, v := range priorMap {
			merged[k] = types.StringValue(v)
		}
	}
	for k, v := range remote {
		merged[k] = types.StringValue(v)
	}

	m, d := types.MapValue(types.StringType, merged)
	diags.Append(d...)
	return m, diags
}
