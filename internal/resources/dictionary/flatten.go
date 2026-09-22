package dictionary

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func flatten(ctx context.Context, d *fastly.Dictionary, m *Model) {
	if d == nil {
		tflog.Warn(ctx, "flatten called with nil dictionary")
		return
	}

	id := fastly.ToValue(d.ServiceID) + "/" + strconv.Itoa(fastly.ToValue(d.ServiceVersion)) + "/" + fastly.ToValue(d.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(d.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(d.ServiceVersion)))

	// Preserve force_destroy from the model before flattening, since it's configuration-only
	// and not returned by the API.
	forceDestroy := m.ForceDestroy
	m.NestedModel = ops{}.ToModel(d)
	if !forceDestroy.IsNull() {
		m.ForceDestroy = forceDestroy
	}

	tflog.Debug(ctx, "Flattened service dictionary state", map[string]any{
		"id":            id,
		"service":       m.Service.ValueString(),
		"version":       m.Version.ValueInt64(),
		"name":          m.Name.ValueString(),
		"dictionary_id": m.DictionaryID.ValueString(),
	})
}
