package header

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func flatten(ctx context.Context, h *fastly.Header, m *Model) {
	if h == nil {
		tflog.Warn(ctx, "flatten called with nil header")
		return
	}

	id := fastly.ToValue(h.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(h.ServiceVersion)) + "-" + fastly.ToValue(h.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(h.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(h.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(h)

	tflog.Debug(ctx, "Flattened service header state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
